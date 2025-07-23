package smsbatch

import (
	"github.com/looplab/fsm"

	"github.com/ashwinyue/dcp/internal/nightwatch/model"
	"github.com/ashwinyue/dcp/internal/nightwatch/service"
	corefsm "github.com/ashwinyue/dcp/internal/nightwatch/watcher/job/smsbatch/core/fsm"
	fsmutil "github.com/ashwinyue/dcp/internal/pkg/util/fsm"
)

// NewStateMachine initializes a new StateMachine with the given initial state, watcher, and SMS batch.
// It configures the FSM with defined events and their corresponding state transitions,
// as well as callbacks for entering specific states.
func NewStateMachine(initial string, watcher *Watcher, smsBatch *model.SmsBatchM) *corefsm.StateMachine {
	// Create table storage service from watcher's store
	tableStorageService := service.NewTableStorageService(watcher.Store.SmsRecord())
	sm := corefsm.NewStateMachine(smsBatch, watcher, tableStorageService)

	// Set the event publisher for the coordinator
	sm.EventCoordinator.SetEventPublisher(watcher.EventPublisher)

	sm.FSM = fsm.NewFSM(
		initial,
		fsm.Events{
			// Define state transitions for the SMS batch process.
			{Name: SmsBatchInitial, Src: []string{SmsBatchInitial}, Dst: SmsBatchPreparationReady},
			{Name: SmsBatchPreparationReady, Src: []string{SmsBatchPreparationReady}, Dst: SmsBatchPreparationRunning},
			{Name: SmsBatchPreparationRunning, Src: []string{SmsBatchPreparationRunning}, Dst: SmsBatchPreparationCompleted},
			{Name: SmsBatchPreparationCompleted, Src: []string{SmsBatchPreparationCompleted}, Dst: SmsBatchDeliveryReady},
			{Name: SmsBatchDeliveryReady, Src: []string{SmsBatchDeliveryReady}, Dst: SmsBatchDeliveryRunning},
			{Name: SmsBatchDeliveryRunning, Src: []string{SmsBatchDeliveryRunning}, Dst: SmsBatchDeliveryCompleted},
			{Name: SmsBatchDeliveryCompleted, Src: []string{SmsBatchDeliveryCompleted}, Dst: SmsBatchSucceeded},

			// Pause and resume transitions
			{Name: SmsBatchPausePreparation, Src: []string{SmsBatchPreparationRunning}, Dst: SmsBatchPreparationPaused},
			{Name: SmsBatchResumePreparation, Src: []string{SmsBatchPreparationPaused}, Dst: SmsBatchPreparationRunning},
			{Name: SmsBatchPauseDelivery, Src: []string{SmsBatchDeliveryRunning}, Dst: SmsBatchDeliveryPaused},
			{Name: SmsBatchResumeDelivery, Src: []string{SmsBatchDeliveryPaused}, Dst: SmsBatchDeliveryRunning},

			// Retry transitions
			{Name: SmsBatchRetryPreparation, Src: []string{SmsBatchPreparationCompleted}, Dst: SmsBatchPreparationReady},
			{Name: SmsBatchRetryDelivery, Src: []string{SmsBatchDeliveryCompleted}, Dst: SmsBatchDeliveryReady},

			// Failure transitions
			{Name: SmsBatchPreparationFailed, Src: []string{SmsBatchPreparationRunning, SmsBatchPreparationReady}, Dst: SmsBatchFailed},
			{Name: SmsBatchDeliveryFailed, Src: []string{SmsBatchDeliveryRunning, SmsBatchDeliveryReady}, Dst: SmsBatchFailed},

			// Abort transition
			{Name: SmsBatchAbort, Src: []string{SmsBatchPreparationRunning, SmsBatchDeliveryRunning}, Dst: SmsBatchAborted},
		},
		fsm.Callbacks{
			// enter_state 先于 enter_xxx 执行
			"enter_state": fsmutil.WrapEvent(sm.EnterState),

			// Initial state callback
			"enter_" + SmsBatchInitial: fsmutil.WrapEvent(sm.InitialExecute),

			// Preparation phase callbacks
			"enter_" + SmsBatchPreparationReady:     fsmutil.WrapEvent(sm.PreparationReady),
			"enter_" + SmsBatchPreparationRunning:   fsmutil.WrapEvent(sm.PreparationExecute),
			"enter_" + SmsBatchPreparationCompleted: fsmutil.WrapEvent(sm.PreparationCompleted),

			// Delivery phase callbacks
			"enter_" + SmsBatchDeliveryReady:     fsmutil.WrapEvent(sm.DeliveryReady),
			"enter_" + SmsBatchDeliveryRunning:   fsmutil.WrapEvent(sm.DeliveryExecute),
			"enter_" + SmsBatchDeliveryCompleted: fsmutil.WrapEvent(sm.DeliveryCompleted),

			// Pause callbacks
			"enter_" + SmsBatchPreparationPaused: fsmutil.WrapEvent(sm.PreparationPause),
			"enter_" + SmsBatchDeliveryPaused:    fsmutil.WrapEvent(sm.DeliveryPause),

			// Resume callbacks - 注意：这里应该使用不同的回调函数
			"after_" + SmsBatchResumePreparation: fsmutil.WrapEvent(sm.PreparationResume),
			"after_" + SmsBatchResumeDelivery:    fsmutil.WrapEvent(sm.DeliveryResume),

			// Final state callbacks
			"enter_" + SmsBatchSucceeded: fsmutil.WrapEvent(sm.BatchSucceeded),
			"enter_" + SmsBatchFailed:    fsmutil.WrapEvent(sm.BatchFailed),
			"enter_" + SmsBatchAborted:   fsmutil.WrapEvent(sm.BatchAborted),
		},
	)

	return sm
}
