package pubnub

import "net/http"

type nonSubMsgType int

const (
	messageTypePublish nonSubMsgType = 1 << iota
	messageTypePAM
)

type JobQResponse struct {
	Resp  *http.Response
	Error error
}

type JobQItem struct {
	Req         *http.Request
	Client      *http.Client
	JobResponse chan *JobQResponse
}

type RequestWorkers struct {
	Workers    chan chan *JobQItem
	MaxWorkers int
	Sem        chan bool
}

type Worker struct {
	Workers    chan chan *JobQItem
	JobChannel chan *JobQItem
	ctx        Context
	id         int
}

var workers []Worker

func newRequestWorkers(workers chan chan *JobQItem, id int, ctx Context) Worker {
	return Worker{
		Workers:    workers,
		JobChannel: make(chan *JobQItem),
		ctx:        ctx,
		id:         id,
	}
}

// Process runs a goroutine for the worker
func (pw Worker) Process(pubnub *PubNub) {
	go func() {
	B:
		for {
			select {
			case pw.Workers <- pw.JobChannel:
				job := <-pw.JobChannel

				if job != nil {

					res, err := job.Client.Do(job.Req)
					jqr := &JobQResponse{
						Error: err,
						Resp:  res,
					}
					job.JobResponse <- jqr
					pubnub.Config.Log.Println("Request sent using worker id ", pw.id)
				}
			case <-pw.ctx.Done():
				pubnub.Config.Log.Println("Exiting Worker Process id ", pw.id)
				break B
			case <-pubnub.ctx.Done():
				pubnub.Config.Log.Println("2 Exiting Worker Process id ", pw.id)
				break B
			}
		}
	}()
}

// Start starts the workers
func (p *RequestWorkers) Start(pubnub *PubNub, ctx Context) {
	pubnub.Config.Log.Println("Start: Running with workers ", p.MaxWorkers)
	workers = make([]Worker, p.MaxWorkers)
	for i := 0; i < p.MaxWorkers; i++ {
		pubnub.Config.Log.Println("Start: StartNonSubWorker ", i)
		worker := newRequestWorkers(p.Workers, i, ctx)
		worker.Process(pubnub)
		workers[i] = worker
	}
	go p.ReadQueue(pubnub)
}

// ReadQueue reads the queue and passes on the job to the workers
func (p *RequestWorkers) ReadQueue(pubnub *PubNub) {
	for job := range pubnub.jobQueue {
		pubnub.Config.Log.Println("ReadQueue: Got job for channel ", job.Req)
		go func(job *JobQItem) {
			jobChannel := <-p.Workers
			jobChannel <- job
		}(job)
	}
	pubnub.Config.Log.Println("ReadQueue: Exit")
}

// Close closes the workers
func (p *RequestWorkers) Close() {

	for _, w := range workers {
		close(w.JobChannel)
		w.ctx.Done()
	}
}
