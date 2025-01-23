package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Engine is a safe task-execution tool for distributing work through redis.
// It uses 3 queues:
// - {job}:tasks - tasks to be picked up by workers
//   - Writer: orchestrator
//   - Reader: workers
//
// - {job}:processing - tasks that were picked up by workers via r.brpoplpush("{job}:tasks", "{job}:processing")
//   - Writer: workers
//   - Reader: orchestrator
//
// - {job}:results - results of tasks
//   - Writer: workers
//   - Reader: orchestrator
//
// The engine receives its tasks from a channel and then writes all the results to the results channel.
// It is NOT safe to have multiple engines touching the same queues, however it is safe to have multiple workers.
//
// The engine takes responsibility to ensuring that if the workers crash, no work is lost (though it may be reordered).
// However, it is the caller's responsibility to ensure that if the engine crashes, all in-progress work will be requeued.
//
// The consumer is expected to read from the results channel as fast as possible. The engine won't read from the input channel
// until the results channel is empty (back-pressure).
//
// The workers MUST use brpoplpush to receive tasks and they MUST push exactly one result per task they process (even if that result is an error).
type EngineTaskMsg struct {
	ID   EngineTaskID `json:"task_id"`
	Task string       `json:"task"`
}
type EngineTaskProcessingMsg struct {
	ID   EngineTaskID `json:"task_id"`
	Task string       `json:"task"`
}
type EngineTaskResultMsg struct {
	ID     EngineTaskID `json:"task_id"`
	Result string       `json:"result"`
}

type QueuedTask struct {
	msg                 EngineTaskMsg
	CreationTime        time.Time
	ProcessingStartTime *time.Time
}

type Engine struct {
	job EngineJobName

	rdb    *redis.Client
	logger *zerolog.Logger

	wg             *sync.WaitGroup
	shouldStopChan chan bool

	// only read from camshaft
	taskInput chan EngineTaskMsg
	// only write from crankshaft
	taskOutput chan EngineTaskResultMsg

	queuedTasksMu sync.Mutex
	queuedTasks   map[EngineTaskID]QueuedTask

	// read-only
	schedulingParams SchedulingParams

	// must not block aquisition of queuedTasksMu
	statsMu sync.Mutex
	stats   []EngineStatEvent
}
type EngineJobName string

const (
	EngineJobNameTest        EngineJobName = "test-engine"
	EngineJobNameInference   EngineJobName = "inference-engine"
	EngineJobNameCompilation EngineJobName = "compilation-engine"
)

type SchedulingParams struct {
	MinTaskQueueSize int
	MaxTaskQueueSize int
	// how long a task can be processing before it is requeued
	TaskProcessingTimeout time.Duration

	// try to keep all of these an order of magnitude
	// or so less than it takes to process a task.
	CamShaftInterval   time.Duration
	CrankShaftInterval time.Duration
	TimingBeltInterval time.Duration
	ODBInterval        time.Duration

	InputChanSize int
	// Even for large numbers, this will still block reading from the input if the consumer is not reading fast enough.
	OutputChanSize int
}

func NewEngine(ctx context.Context, job EngineJobName, rdb *redis.Client, schedulingParams SchedulingParams) *Engine {
	parentLogger := zerolog.Ctx(ctx)
	logger := parentLogger.With().Str("job", string(job)).Logger()

	return &Engine{
		job:              job,
		rdb:              rdb,
		logger:           &logger,
		wg:               &sync.WaitGroup{},
		shouldStopChan:   make(chan bool),
		taskInput:        make(chan EngineTaskMsg, schedulingParams.InputChanSize),
		taskOutput:       make(chan EngineTaskResultMsg, schedulingParams.OutputChanSize),
		queuedTasksMu:    sync.Mutex{},
		queuedTasks:      make(map[EngineTaskID]QueuedTask),
		schedulingParams: schedulingParams,
	}
}

func (e *Engine) Start(ctx context.Context) error {
	e.logger.Debug().Msg("Starting engine")

	err := e.dropQueuesForStartup(ctx)
	if err != nil {
		return err
	}
	e.wg.Add(4)
	go e.createCamshaft()
	go e.createCrankshaft()
	go e.createTimingBelt()
	go e.createOBD()

	return nil
}
func (e *Engine) dropQueuesForStartup(ctx context.Context) error {
	e.logger.Debug().Msg("Dropping queues for startup")
	err := e.rdb.Del(ctx, e.TasksQueueName()).Err()
	if err != nil {
		return err
	}
	err = e.rdb.Del(ctx, e.ProcessingQueueName()).Err()
	if err != nil {
		return err
	}
	err = e.rdb.Del(ctx, e.ResultsQueueName()).Err()
	if err != nil {
		return err
	}
	return nil
}

func (e *Engine) TriggerStop() {
	close(e.shouldStopChan)
}

func (e *Engine) WaitForStop() {
	e.logger.Info().Msg("Waiting for engine to stop")
	e.wg.Wait()
}

func (e *Engine) TasksQueueName() string {
	return fmt.Sprintf("%s:tasks", e.job)
}
func (e *Engine) ProcessingQueueName() string {
	return fmt.Sprintf("%s:processing", e.job)
}
func (e *Engine) ResultsQueueName() string {
	return fmt.Sprintf("%s:results", e.job)
}
func (e *Engine) GetInput() chan<- EngineTaskMsg {
	return e.taskInput
}
func (e *Engine) GetOutput() <-chan EngineTaskResultMsg {
	return e.taskOutput
}

func (e *Engine) setupLoggerAndCtxForComponent(component string) (context.Context, context.CancelFunc, *zerolog.Logger) {
	logger := e.logger.With().Str("component", component).Logger()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.WithContext(ctx)
	return ctx, cancel, &logger
}

// the camshaft handles the tasks chan -> tasks queue.
func (e *Engine) createCamshaft() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.schedulingParams.CamShaftInterval)
	ctx, cancel, logger := e.setupLoggerAndCtxForComponent("camshaft")
	defer cancel()
	for {
		select {
		case <-e.shouldStopChan:
			logger.Debug().Msg("Camshaft stopping")
			return
		case <-ticker.C:
		}
		startTime := time.Now()

		e.recordStatEvent(EngineStatEvent{
			camshaftStarted: 1,
		})

		// requeue tasks that have been processing for too long.
		// We do this before enqueing new tasks to avoid over-filling the queue.
		func() {
			e.queuedTasksMu.Lock()
			defer e.queuedTasksMu.Unlock()
			numTasksRequeued := 0

			for _, task := range e.queuedTasks {
				if task.ProcessingStartTime != nil && time.Since(*task.ProcessingStartTime) > e.schedulingParams.TaskProcessingTimeout {
					taskMsg := task.msg.toJSON()
					err := e.rdb.LPush(ctx, e.TasksQueueName(), taskMsg).Err()
					if err != nil {
						logger.Error().Err(err).Msg("Failed to requeue timed out task")
						continue
					}
					// reset processing start time
					task.ProcessingStartTime = nil
					e.queuedTasks[task.msg.ID] = task
					numTasksRequeued++
				}
			}
			logger.Debug().Msgf("Requeued %d tasks", numTasksRequeued)
			e.recordStatEvent(EngineStatEvent{
				tasksRequeued: numTasksRequeued,
			})
		}()

		// the job of this routine is to keep this queue fed. Not to care about the
		// fake e.queuedTasks which is just a monitoring tool and doesn't have to be strictly accurate
		tasksQueueSize, err := e.rdb.LLen(ctx, e.TasksQueueName()).Result()
		if err != nil {
			logger.Error().Err(err).Msg("Error getting tasks queue size")
			continue
		}
		if tasksQueueSize > int64(e.schedulingParams.MinTaskQueueSize) {
			// Don't do anything if we have enough tasks.
			continue
		}
		if len(e.taskOutput) > 0 {
			// Don't do anything if the consumer is still catching up.
			// This is a back-pressure mechanism.
			logger.Debug().Msg("Camshaft skipping adding tasks because the consumer is still catching up")
			e.recordStatEvent(EngineStatEvent{
				camshaftBlockedFromBackpressure: 1,
			})
			continue
		}
		numTasksToAdd := e.schedulingParams.MaxTaskQueueSize - int(tasksQueueSize)
		tasks := make([]EngineTaskMsg, 0, numTasksToAdd)
		for i := 0; i < numTasksToAdd; i++ {
			select {
			case <-e.shouldStopChan:
				logger.Debug().Msg("Camshaft stopping without adding any tasks")
				return
			case task, ok := <-e.taskInput:
				if !ok {
					logger.Fatal().Msg("Engine input channel should never be closed")
				}
				if task.ID == "" {
					task.ID = NewEngineTaskID()
				}
				tasks = append(tasks, task)
			default:
				// no task to add.
			}
		}

		func() {
			e.queuedTasksMu.Lock()
			defer e.queuedTasksMu.Unlock()

			lastK := 0
			for _, msg := range tasks {
				e.queuedTasks[msg.ID] = QueuedTask{
					msg:                 msg,
					CreationTime:        time.Now(),
					ProcessingStartTime: nil,
				}
				msg := msg.toJSON()
				// Could be done outside the lock. Optimize if needed.
				res := e.rdb.LPush(ctx, e.TasksQueueName(), msg)
				if res.Err() != nil {
					// This is non-fatal because the task will get requeued.
					logger.Error().Err(res.Err()).Msg("Error pushing tasks to queue")
					continue
				}
				lastK = int(res.Val())
			}
			logger.Debug().Msgf("Pushed %d tasks to queue. Queue size: %d", len(tasks), lastK)
		}()
		e.recordStatEvent(EngineStatEvent{
			camshaftExecuted:      1,
			camshaftExecutionTime: time.Since(startTime),
		})
	}

}

// the crank shaft handles the results queue -> results chan.
func (e *Engine) createCrankshaft() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.schedulingParams.CrankShaftInterval)
	ctx, cancel, logger := e.setupLoggerAndCtxForComponent("crankshaft")
	defer cancel()
	for {
		select {
		case <-e.shouldStopChan:
			logger.Debug().Msg("Crankshaft stopping")
			return
		case <-ticker.C:
		}
		startTime := time.Now()

		e.recordStatEvent(EngineStatEvent{
			crankshaftStarted: 1,
		})

		resultsQueueSize, err := e.rdb.LLen(ctx, e.ResultsQueueName()).Result()
		if err != nil {
			logger.Error().Err(err).Msg("Error getting results queue size")
			continue
		}
		if resultsQueueSize == 0 {
			continue
		}
		resultsToSend := make([]EngineTaskResultMsg, 0, resultsQueueSize)
		results := make([]EngineTaskResultMsg, 0, resultsQueueSize)
		for i := 0; i < int(resultsQueueSize); i++ {
			m, err := e.rdb.BRPop(ctx, 5*time.Second, e.ResultsQueueName()).Result()
			if err != nil {
				logger.Error().Err(err).Msg("Error popping results from queue")
				continue
			}
			resultMsg, err := engineTaskResultMsgFromJSON(m[1])
			if err != nil {
				logger.Error().Err(err).Msg("Error unmarshalling result message")
				continue
			}
			results = append(results, *resultMsg)
		}

		func() {
			e.queuedTasksMu.Lock()
			defer e.queuedTasksMu.Unlock()

			for _, result := range results {
				queuedTask, ok := e.queuedTasks[result.ID]
				if !ok {
					logger.Warn().Msgf("Found result for task that is not in the queue: %+v", result)
					continue
				}
				if queuedTask.ProcessingStartTime != nil {
					e.recordStatEvent(EngineStatEvent{
						tasksFinished:      1,
						taskFinishedInTime: time.Since(*queuedTask.ProcessingStartTime),
					})
				} else {
					logger.Warn().Msgf("Found result for task that has no processing start time: %+v. This likely means the timing belt is not running fast enough", result)
					logger.Warn().Msgf("Timing belt interval: %s. This will mess with the stats.", e.schedulingParams.TimingBeltInterval)
					e.recordStatEvent(EngineStatEvent{
						tasksFinished:      1,
						taskFinishedInTime: e.schedulingParams.TimingBeltInterval,
					})
				}
				delete(e.queuedTasks, result.ID)
				resultsToSend = append(resultsToSend, result)
			}
		}()
		logger.Debug().Msgf("Sending %d results to output channel", len(resultsToSend))
		for _, result := range resultsToSend {
			select {
			case <-e.shouldStopChan:
				logger.Warn().Msg("Crankshaft stopping without sending all results")
				return
			case e.taskOutput <- result:
			}
		}
		e.recordStatEvent(EngineStatEvent{
			crankshaftExecuted:      1,
			crankshaftExecutionTime: time.Since(startTime),
		})
	}
}

// the timing belt runs the processing queue -> updating queuedTasks goroutine
// these jobs are then requeued in the cam shaft.
// (the engine analogy doesn't work very well with this one... open to advice).
func (e *Engine) createTimingBelt() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.schedulingParams.TimingBeltInterval)
	ctx, cancel, logger := e.setupLoggerAndCtxForComponent("timing belt")
	defer cancel()
	for {
		select {
		case <-e.shouldStopChan:
			logger.Debug().Msg("Timing belt stopping")
			return
		case <-ticker.C:
		}
		startTime := time.Now()

		e.recordStatEvent(EngineStatEvent{
			timingBeltStarted: 1,
		})

		// This isn't accurate. However, jobs should be expected to take significantly longer than the timing belt interval.
		processingQueueSize, err := e.rdb.LLen(ctx, e.ProcessingQueueName()).Result()
		if err != nil {
			logger.Error().Err(err).Msg("Error getting processing queue size")
			continue
		}
		if processingQueueSize == 0 {
			continue
		}
		processingMsgs := make([]EngineTaskMsg, 0, processingQueueSize)
		for i := 0; i < int(processingQueueSize); i++ {
			m, err := e.rdb.BRPop(ctx, 5*time.Second, e.ProcessingQueueName()).Result()
			if err != nil {
				logger.Error().Err(err).Msg("Error popping processing from queue")
				continue
			}
			msg, err := engineTaskMsgFromJSON(m[1])
			if err != nil {
				// This is fatal because the task won't get requeued and will be lost.
				logger.Fatal().Err(err).Msg("Error unmarshalling processing message")
			}
			processingMsgs = append(processingMsgs, *msg)
		}

		func() {
			e.queuedTasksMu.Lock()
			defer e.queuedTasksMu.Unlock()

			for _, msg := range processingMsgs {
				task, ok := e.queuedTasks[msg.ID]
				if !ok {
					logger.Warn().Msgf("Found processing message for task that is not in the queue: %+v", msg)
					continue
				}

				e.recordStatEvent(EngineStatEvent{
					taskTimeSpentInQueue: startTime.Sub(task.CreationTime),
				})

				// use job start time to reduce the impact of reading from the queue & from waiting on the lock.
				task.ProcessingStartTime = &startTime
				e.queuedTasks[msg.ID] = task
			}
		}()
		e.recordStatEvent(EngineStatEvent{
			timingBeltExecuted:      1,
			timingBeltExecutionTime: time.Since(startTime),
		})
	}
}

// the odb emits stats to the logger.
func (e *Engine) createOBD() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.schedulingParams.ODBInterval)
	_, cancel, logger := e.setupLoggerAndCtxForComponent("odb")
	defer cancel()
	for {
		select {
		case <-e.shouldStopChan:
			logger.Debug().Msg("OBD stopping")
			return
		case <-ticker.C:
		}
		startTime := time.Now()

		e.recordStatEvent(EngineStatEvent{
			odbStarted: 1,
		})
		statsLines := []string{}
		statsLines = append(statsLines, "Emitting stats")
		mergedStats := e.mergeStatsInInterval(nil)
		statsLines = append(statsLines, fmt.Sprintf("\tNum stat events: %d", mergedStats.NumEvents))
		statsLines = append(statsLines, fmt.Sprintf("\tTasks finished: %d", mergedStats.tasksFinished))
		statsLines = append(statsLines, fmt.Sprintf("\tAvg processing time per task: %s", mergedStats.AvgProcessingTimePerTask))
		statsLines = append(statsLines, "")
		statsLines = append(statsLines, fmt.Sprintf("\tTasks requeued: %d", mergedStats.tasksRequeued))
		statsLines = append(statsLines, fmt.Sprintf("\tAvg task time spent in queue: %s", mergedStats.AvgTaskTimeSpentInQueue))
		statsLines = append(statsLines, "")
		statsLines = append(statsLines, fmt.Sprintf("\tCamshaft blocked from backpressure: %d", mergedStats.camshaftBlockedFromBackpressure))
		statsLines = append(statsLines, fmt.Sprintf("\tCamshaft started: %d", mergedStats.camshaftStarted))
		statsLines = append(statsLines, fmt.Sprintf("\tCamshaft executed: %d", mergedStats.camshaftExecuted))
		statsLines = append(statsLines, fmt.Sprintf("\tCamshaft execution time: %s", mergedStats.camshaftExecutionTime))
		statsLines = append(statsLines, "")
		statsLines = append(statsLines, fmt.Sprintf("\tCrankshaft started: %d", mergedStats.crankshaftStarted))
		statsLines = append(statsLines, fmt.Sprintf("\tCrankshaft executed: %d", mergedStats.crankshaftExecuted))
		statsLines = append(statsLines, fmt.Sprintf("\tCrankshaft execution time: %s", mergedStats.crankshaftExecutionTime))
		statsLines = append(statsLines, "")
		statsLines = append(statsLines, fmt.Sprintf("\tTiming belt started: %d", mergedStats.timingBeltStarted))
		statsLines = append(statsLines, fmt.Sprintf("\tTiming belt executed: %d", mergedStats.timingBeltExecuted))
		statsLines = append(statsLines, fmt.Sprintf("\tTiming belt execution time: %s", mergedStats.timingBeltExecutionTime))
		statsLines = append(statsLines, "")
		statsLines = append(statsLines, fmt.Sprintf("\tOBD started: %d", mergedStats.odbStarted))
		statsLines = append(statsLines, fmt.Sprintf("\tOBD executed: %d", mergedStats.odbExecuted))
		statsLines = append(statsLines, fmt.Sprintf("\tOBD execution time: %s", mergedStats.odbExecutionTime))
		statsLines = append(statsLines, "")
		logger.Debug().Msg(strings.Join(statsLines, "\n"))

		e.recordStatEvent(EngineStatEvent{
			odbExecuted:      1,
			odbExecutionTime: time.Since(startTime),
		})
	}
}

// EngineStatEvent is a single stat event.
// It is important that the null-value indicates that the stat is not set.
// 🚩 If you add a new stat, make sure to add it to the mergedStatsInInterval function & the OBD.
type EngineStatEvent struct {
	timestamp                       time.Time
	tasksFinished                   int
	taskFinishedInTime              time.Duration
	tasksRequeued                   int
	taskTimeSpentInQueue            time.Duration
	camshaftBlockedFromBackpressure int
	// started == was scheduled
	camshaftStarted int
	// executed == did something
	camshaftExecuted        int
	camshaftExecutionTime   time.Duration
	crankshaftStarted       int
	crankshaftExecuted      int
	crankshaftExecutionTime time.Duration
	timingBeltStarted       int
	timingBeltExecuted      int
	timingBeltExecutionTime time.Duration
	odbStarted              int
	odbExecuted             int
	odbExecutionTime        time.Duration
}

type MergedStats struct {
	EngineStatEvent
	NumEvents                int
	AvgProcessingTimePerTask time.Duration
	AvgTaskTimeSpentInQueue  time.Duration
}

func (e *Engine) mergeStatsInInterval(interval *time.Duration) MergedStats {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()

	startIndex := 0
	if interval != nil {
		startIndex = sort.Search(len(e.stats), func(i int) bool {
			return e.stats[i].timestamp.Add(*interval).After(time.Now())
		})
	}
	mergedStats := MergedStats{}
	mergedEvent := EngineStatEvent{}
	for i := startIndex; i < len(e.stats); i++ {
		mergedStats.NumEvents++
		event := e.stats[i]
		mergedEvent.tasksFinished += event.tasksFinished
		mergedEvent.taskFinishedInTime += event.taskFinishedInTime
		mergedEvent.tasksRequeued += event.tasksRequeued
		mergedEvent.taskTimeSpentInQueue += event.taskTimeSpentInQueue
		mergedEvent.camshaftBlockedFromBackpressure += event.camshaftBlockedFromBackpressure
		mergedEvent.camshaftStarted += event.camshaftStarted
		mergedEvent.camshaftExecuted += event.camshaftExecuted
		mergedEvent.camshaftExecutionTime += event.camshaftExecutionTime
		mergedEvent.crankshaftStarted += event.crankshaftStarted
		mergedEvent.crankshaftExecuted += event.crankshaftExecuted
		mergedEvent.crankshaftExecutionTime += event.crankshaftExecutionTime
		mergedEvent.timingBeltStarted += event.timingBeltStarted
		mergedEvent.timingBeltExecuted += event.timingBeltExecuted
		mergedEvent.timingBeltExecutionTime += event.timingBeltExecutionTime
		mergedEvent.odbStarted += event.odbStarted
		mergedEvent.odbExecuted += event.odbExecuted
		mergedEvent.odbExecutionTime += event.odbExecutionTime
	}
	mergedStats.EngineStatEvent = mergedEvent
	if mergedEvent.tasksFinished > 0 {
		mergedStats.AvgProcessingTimePerTask = mergedEvent.taskFinishedInTime / time.Duration(mergedEvent.tasksFinished)
		mergedStats.AvgTaskTimeSpentInQueue = mergedEvent.taskTimeSpentInQueue / time.Duration(mergedEvent.tasksFinished)
	}
	return mergedStats
}

func (e *Engine) recordStatEvent(event EngineStatEvent) {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()
	// it is important that the timestamp is calculated with the lock so the stats are in order
	// (even though that slightly messes with the stats)
	event.timestamp = time.Now()
	e.stats = append(e.stats, event)
}

func (t *EngineTaskMsg) toJSON() string {
	msg, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(msg)
}

func engineTaskMsgFromJSON(input string) (*EngineTaskMsg, error) {
	var msg EngineTaskMsg
	err := json.Unmarshal([]byte(input), &msg)
	if err != nil {
		return nil, err
	}
	// sanity check
	if !IsValidEngineTaskID(msg.ID) {
		return nil, fmt.Errorf("invalid engine task id: %s", msg.ID)
	}
	return &msg, nil
}

func engineTaskResultMsgFromJSON(input string) (*EngineTaskResultMsg, error) {
	var msg EngineTaskResultMsg
	err := json.Unmarshal([]byte(input), &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
