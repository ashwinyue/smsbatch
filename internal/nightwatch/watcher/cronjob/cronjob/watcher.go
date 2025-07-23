package cronjob

import (
	"context"
	"fmt"
	"strings"

	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/onexstack/onexstack/pkg/watch/manager"
	"github.com/onexstack/onexstack/pkg/watch/registry"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	known "github.com/ashwinyue/dcp/internal/pkg/known/nightwatch"
)

const CronJobMPrefix = "nightwatch_cronjob/"

// Watcher implements cronjob watcher
type Watcher struct {
	store store.IStore
	jm    *manager.JobManager
}

// Ensure Watcher implements required interfaces
var _ registry.Watcher = (*Watcher)(nil)

// Run executes the watcher logic
func (w *Watcher) Run() {
	ctx := context.Background()

	_, cronjobs, err := w.store.CronJob().List(ctx, where.F("suspend", known.JobNonSuspended))
	if err != nil {
		return
	}

	w.removeNonExistentCronJobs(cronjobs)

	for _, cronjob := range cronjobs {
		w.processCronJob(ctx, cronjob)
	}
}

// processCronJob processes a single cronjob
func (w *Watcher) processCronJob(ctx context.Context, cronjob *model.CronJobM) {
	jobName := cronJobName(cronjob.CronJobID)

	// Check if job template is valid
	if !w.hasValidTemplate(cronjob) {
		return
	}

	// Skip if job already exists
	if w.jm.JobExists(jobName) {
		return
	}

	// Create save job
	saveJob := &saveJob{
		ctx:     ctx,
		store:   w.store,
		cronJob: cronjob,
	}

	w.jm.AddJob(jobName, cronjob.Schedule, saveJob)
}

// hasValidTemplate checks if CronJob has a valid job template
func (w *Watcher) hasValidTemplate(cronjob *model.CronJobM) bool {
	return cronjob.SmsBatchTemplate != nil
}

// removeNonExistentCronJobs removes Cron jobs from the scheduler that no longer exist
func (w *Watcher) removeNonExistentCronJobs(cronjobs []*model.CronJobM) {
	validCronJobIDs := make(map[string]struct{}, len(cronjobs))
	for _, cronjob := range cronjobs {
		validCronJobIDs[cronJobName(cronjob.CronJobID)] = struct{}{}
	}

	for jobName := range w.jm.GetJobs() {
		// Only process cronjob names
		if !isCronJobMName(jobName) {
			continue
		}

		if _, exists := validCronJobIDs[jobName]; exists {
			continue
		}

		w.jm.RemoveJob(jobName)
	}
}

// Spec returns the cron specification for this watcher
func (w *Watcher) Spec() string {
	return "@every 1s"
}

// SetStore sets the persistence store for the Watcher
func (w *Watcher) SetStore(store store.IStore) {
	w.store = store
}

// SetJobManager sets the JobManager for the Watcher
func (w *Watcher) SetJobManager(jm *manager.JobManager) {
	w.jm = jm
}

// saveJob implements the job that creates SMS batches
type saveJob struct {
	ctx     context.Context
	store   store.IStore
	cronJob *model.CronJobM
}

func (j *saveJob) Run() {
	j.createSmsBatch()
}

// createSmsBatch creates SmsBatch job
func (j *saveJob) createSmsBatch() {
	// Check existing batches
	count, _, err := j.store.SmsBatch().List(j.ctx, where.F("cronjob_id", j.cronJob.CronJobID))
	if err != nil || count >= known.MaxJobsPerCronJob {
		return
	}

	// Validate template
	if j.cronJob.SmsBatchTemplate == nil {
		return
	}

	// Create new batch
	smsBatch := j.cronJob.SmsBatchTemplate
	smsBatch.ID = 0 // Reset ID to prevent conflicts
	smsBatch.UserID = j.cronJob.UserID
	smsBatch.Scope = j.cronJob.Scope
	smsBatch.Name = fmt.Sprintf("smsbatch-for-%s", j.cronJob.Name)
	smsBatch.CronJobID = &j.cronJob.CronJobID

	_ = j.store.SmsBatch().Create(j.ctx, smsBatch)
}

func cronJobName(cronJobID string) string {
	return fmt.Sprintf("%s%s", CronJobMPrefix, cronJobID)
}

func isCronJobMName(jobName string) bool {
	return strings.HasPrefix(jobName, CronJobMPrefix)
}

func init() {
	registry.Register("cronjob", &Watcher{})
}
