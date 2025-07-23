package cronjob

import (
	"context"
	"fmt"

	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/onexstack/onexstack/pkg/watch/manager"
	"github.com/onexstack/onexstack/pkg/watch/registry"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	"github.com/ashwinyue/dcp/internal/nightwatch/watcher"
	known "github.com/ashwinyue/dcp/internal/pkg/known/nightwatch"
	"github.com/ashwinyue/dcp/internal/pkg/log"
)

// EnhancedWatcher is an improved version of the cronjob watcher with monitoring and resilience
type EnhancedWatcher struct {
	*watcher.EnhancedWatcher
	jm *manager.JobManager
}

// Ensure EnhancedWatcher implements required interfaces
var _ registry.Watcher = (*EnhancedWatcher)(nil)
var _ watcher.WantsStore = (*EnhancedWatcher)(nil)
var _ watcher.WantsEnhancedFeatures = (*EnhancedWatcher)(nil)

// NewEnhancedWatcher creates a new enhanced cronjob watcher
func NewEnhancedWatcher() *EnhancedWatcher {
	return &EnhancedWatcher{
		EnhancedWatcher: watcher.NewEnhancedWatcher("enhanced-cronjob"),
	}
}

// Run executes the watcher logic with enhanced error handling and monitoring
func (w *EnhancedWatcher) Run() {
	ctx := context.Background()
	
	// Use the enhanced run method with resilience features
	w.RunWithResilience(ctx, w.runLogic)
}

// runLogic contains the actual watcher logic
func (w *EnhancedWatcher) runLogic(ctx context.Context) error {
	store := w.GetStore()
	if store == nil {
		return watcher.NewRetryableError(fmt.Errorf("store not initialized"), false)
	}

	_, cronjobs, err := store.CronJob().List(ctx, where.F("suspend", known.JobNonSuspended))
	if err != nil {
		return watcher.NewRetryableError(fmt.Errorf("failed to list cronjobs: %w", err), true)
	}

	if err := w.removeNonExistentCronJobs(cronjobs); err != nil {
		log.Warnw("Failed to remove non-existent cronjobs", "error", err)
		// Continue processing even if cleanup fails
	}

	for _, cronjob := range cronjobs {
		if err := w.processCronJob(ctx, cronjob); err != nil {
			log.Errorw("Failed to process cronjob", "cronjob_id", cronjob.CronJobID, "error", err)
			// Continue with other cronjobs even if one fails
		}
	}

	return nil
}

// processCronJob processes a single cronjob
func (w *EnhancedWatcher) processCronJob(ctx context.Context, cronjob *model.CronJobM) error {
	jobName := cronJobName(cronjob.CronJobID)

	// Check if job template is valid
	if !w.hasValidTemplate(cronjob) {
		return watcher.NewRetryableError(
			fmt.Errorf("cronjob %s has no valid template", cronjob.CronJobID),
			false, // Template issues are not retryable
		)
	}

	// Skip if job already exists
	if w.jm.JobExists(jobName) {
		return nil
	}

	// Create save job with enhanced error handling
	saveJob := &enhancedSaveJob{
		ctx:     ctx,
		store:   w.GetStore(),
		cronJob: cronjob,
	}

	w.jm.AddJob(jobName, cronjob.Schedule, saveJob)
	log.Infow("Added cronjob to scheduler", "cronjob_id", cronjob.CronJobID, "schedule", cronjob.Schedule)

	return nil
}

// hasValidTemplate checks if CronJob has a valid job template
func (w *EnhancedWatcher) hasValidTemplate(cronjob *model.CronJobM) bool {
	return cronjob.SmsBatchTemplate != nil
}

// removeNonExistentCronJobs removes Cron jobs from the scheduler that no longer exist
func (w *EnhancedWatcher) removeNonExistentCronJobs(cronjobs []*model.CronJobM) error {
	validCronJobIDs := make(map[string]struct{}, len(cronjobs))
	for _, cronjob := range cronjobs {
		validCronJobIDs[cronJobName(cronjob.CronJobID)] = struct{}{}
	}

	var errors []error
	for jobName := range w.jm.GetJobs() {
		// Only process cronjob names
		if !isCronJobMName(jobName) {
			continue
		}

		if _, exists := validCronJobIDs[jobName]; exists {
			continue
		}

		if err := w.jm.RemoveJob(jobName); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove job %s: %w", jobName, err))
		} else {
			log.Infow("Removed obsolete cronjob from scheduler", "job_name", jobName)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to remove some jobs: %v", errors)
	}
	return nil
}

// Spec returns the cron specification for this watcher
func (w *EnhancedWatcher) Spec() string {
	return "@every 1s"
}

// SetJobManager sets the JobManager for the Watcher
func (w *EnhancedWatcher) SetJobManager(jm *manager.JobManager) {
	w.jm = jm
}

// enhancedSaveJob is an improved version of saveJob with better error handling
type enhancedSaveJob struct {
	ctx     context.Context
	store   store.IStore
	cronJob *model.CronJobM
}

func (j *enhancedSaveJob) Run() {
	if err := j.createSmsBatch(); err != nil {
		log.Errorw("Failed to create SMS batch from cronjob",
			"cronjob_id", j.cronJob.CronJobID,
			"error", err,
		)
	}
}

// createSmsBatch creates SmsBatch job with enhanced error handling
func (j *enhancedSaveJob) createSmsBatch() error {
	// Check existing batches
	count, _, err := j.store.SmsBatch().List(j.ctx, where.F("cronjob_id", j.cronJob.CronJobID))
	if err != nil {
		return fmt.Errorf("failed to list existing sms batches: %w", err)
	}

	if count >= known.MaxJobsPerCronJob {
		return fmt.Errorf("max sms batches per cronjob reached: %d", count)
	}

	// Validate template
	if j.cronJob.SmsBatchTemplate == nil {
		return fmt.Errorf("SmsBatchTemplate is nil")
	}

	// Create new batch
	smsBatch := j.cronJob.SmsBatchTemplate
	smsBatch.ID = 0 // Reset ID to prevent conflicts
	smsBatch.UserID = j.cronJob.UserID
	smsBatch.Scope = j.cronJob.Scope
	smsBatch.Name = fmt.Sprintf("smsbatch-for-%s", j.cronJob.Name)
	smsBatch.CronJobID = &j.cronJob.CronJobID

	if err := j.store.SmsBatch().Create(j.ctx, smsBatch); err != nil {
		return fmt.Errorf("failed to create sms batch: %w", err)
	}

	log.Infow("Created sms batch from cronjob",
		"cronjob_id", j.cronJob.CronJobID,
		"batch_id", smsBatch.BatchID,
		"batch_name", smsBatch.Name,
	)

	return nil
}

// cronJobName and isCronJobMName functions are already defined in watcher.go

func init() {
	registry.Register("enhanced-cronjob", NewEnhancedWatcher())
}