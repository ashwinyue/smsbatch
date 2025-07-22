// Package cronjob is a watcher implement.
package cronjob

import (
	"context"
	"fmt"
	"strings"

	"github.com/onexstack/onexstack/pkg/log"
	"github.com/onexstack/onexstack/pkg/store/where"
	"github.com/onexstack/onexstack/pkg/watch/manager"
	"github.com/onexstack/onexstack/pkg/watch/registry"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/store"
	known "github.com/ashwinyue/dcp/internal/pkg/known/nightwatch"
)

var _ registry.Watcher = (*Watcher)(nil)

const CronJobMPrefix = "nightwatch_cronjob/"

// watcher implement.
type Watcher struct {
	store store.IStore
	jm    *manager.JobManager
}

type saveJob struct {
	ctx     context.Context
	store   store.IStore
	cronJob *model.CronJobM
}

func (j saveJob) Run() {
	// 只支持SmsBatch任务类型
	j.createSmsBatch()
}

// createSmsBatch 创建SmsBatch任务
func (j saveJob) createSmsBatch() {
	count, _, err := j.store.SmsBatch().List(j.ctx, where.F("cronjob_id", j.cronJob.CronJobID))
	if err != nil {
		log.Errorw(err, "Failed to list sms batches")
		return
	}
	if count >= known.MaxJobsPerCronJob { // 复用相同的限制
		log.Warnw("Max sms batches per cronjob reached", "cronjob_id", j.cronJob.CronJobID)
		return
	}

	// 检查SmsBatchTemplate是否存在
	if j.cronJob.SmsBatchTemplate == nil {
		log.Errorw(nil, "SmsBatchTemplate is nil", "cronjob_id", j.cronJob.CronJobID)
		return
	}

	// To prevent primary key conflicts.
	smsBatch := j.cronJob.SmsBatchTemplate
	smsBatch.ID = 0
	smsBatch.UserID = j.cronJob.UserID
	smsBatch.Scope = j.cronJob.Scope
	smsBatch.Name = fmt.Sprintf("smsbatch-for-%s", j.cronJob.Name)

	// 设置CronJobID字段
	smsBatch.CronJobID = &j.cronJob.CronJobID

	if err := j.store.SmsBatch().Create(j.ctx, smsBatch); err != nil {
		log.Errorw(err, "Failed to create sms batch")
		return
	}

	log.Infow("Created sms batch from cronjob", "cronjob_id", j.cronJob.CronJobID, "batch_id", smsBatch.BatchID)
}

// Run runs the watcher.
func (w *Watcher) Run() {
	ctx := context.Background()
	_, cronjobs, err := w.store.CronJob().List(ctx, where.F("suspend", known.JobNonSuspended))
	if err != nil {
		return
	}

	w.RemoveNonExistentCronJobs(cronjobs)

	for _, cronjob := range cronjobs {
		jobName := cronJobName(cronjob.CronJobID)
		//ctx = log.WithContext(ctx, "cronjob_id", cronjob.CronJobID)

		// 检查任务模板是否存在
		if !w.hasValidTemplate(cronjob) {
			continue
		}

		if w.jm.JobExists(jobName) {
			continue
		}

		w.jm.AddJob(jobName, cronjob.Schedule, saveJob{store: w.store, ctx: ctx, cronJob: cronjob})
	}
}

// hasValidTemplate 检查CronJob是否有有效的任务模板
func (w *Watcher) hasValidTemplate(cronjob *model.CronJobM) bool {
	return cronjob.SmsBatchTemplate != nil
}

// RemoveNonExistentCronJobs removes Cron jobs from the scheduler that no longer exist.
func (w *Watcher) RemoveNonExistentCronJobs(cronjobs []*model.CronJobM) {
	validCronJobIDs := make(map[string]struct{}, len(cronjobs))
	for _, cronjob := range cronjobs {
		validCronJobIDs[cronJobName(cronjob.CronJobID)] = struct{}{}
	}

	for jobName := range w.jm.GetJobs() {
		// Note: Do not delete CronJobs that are not walle_cronjobs here.
		if !isCronJobMName(jobName) {
			continue
		}

		if _, exists := validCronJobIDs[jobName]; exists {
			continue
		}
		_ = w.jm.RemoveJob(jobName)
	}
}

// Spec is parsed using the time zone of task Cron instance as the default.
func (w *Watcher) Spec() string {
	return "@every 1s"
}

// SetStore sets the persistence store for the Watcher.
func (w *Watcher) SetStore(store store.IStore) {
	w.store = store
}

// SetJobManager sets the JobManager for the Watcher.
func (w *Watcher) SetJobManager(jm *manager.JobManager) {
	w.jm = jm
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
