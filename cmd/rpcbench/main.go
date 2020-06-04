package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/anycable/anycable-go/common"
	"github.com/anycable/anycable-go/metrics"
	"github.com/anycable/anycable-go/stats"
	nanoid "github.com/matoous/go-nanoid"

	"github.com/golang-collections/go-datastructures/queue"

	"github.com/anycable/anycable-go/rpc"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"

	"github.com/namsral/flag"
)

// Options describes benchmark options
type Options struct {
	host        string
	capacity    int
	concurrency int
	total       int
	channel     string
	action      string
	mode        string
	debug       bool
}

// Benchmark implements shared methods for the benchmark
type Benchmark struct {
	rpc     *rpc.Controller
	perform bool
	channel string
	action  string
	metrics *metrics.Metrics
	buffer  *queue.RingBuffer
	resChan chan (time.Duration)
	errChan chan (error)
}

func main() {
	var b Benchmark
	var options Options

	parseOptions(&options)

	// init logging
	logLevel := "info"

	if options.debug {
		logLevel = "debug"
	}

	err := utils.InitLogger("text", logLevel)

	if err != nil {
		log.Errorf("!!! Failed to initialize logger !!!\n%v", err)
		os.Exit(1)
	}

	log.WithField(
		"concurrency",
		options.concurrency,
	).WithField(
		"total",
		options.total,
	).WithField(
		"capacity",
		options.capacity,
	).WithField(
		"mode",
		options.mode,
	).Infof("Running RPC benchmark for %s", options.host)

	channel, _ := json.Marshal(map[string]string{"channel": options.channel})
	b.channel = string(channel)
	action, _ := json.Marshal(map[string]string{"action": options.action})
	b.action = string(action)

	rpcConfig := rpc.NewConfig()
	rpcConfig.Concurrency = options.capacity
	rpcConfig.Host = options.host

	b.perform = options.mode == "perform"
	b.metrics = metrics.NewMetrics(nil, 0)
	b.rpc = rpc.NewController(b.metrics, &rpcConfig)

	err = b.rpc.Start()

	if err != nil {
		panic("Failed to initialize RPC")
	}

	b.resChan = make(chan time.Duration, 1000)
	b.errChan = make(chan error, 1000)
	b.buffer = queue.NewRingBuffer(uint64(options.total))

	for i := 0; i < options.total; i++ {
		if err = b.buffer.Put(nil); err != nil {
			panic(err)
		}
	}

	for i := 0; i < options.concurrency; i++ {
		go b.startWorker()
	}

	var resAgg stats.ResAggregate

	completed := 0
	failures := 0

	for completed < options.total {
		select {
		case result := <-b.resChan:
			resAgg.Add(result)
		case err := <-b.errChan:
			log.Warnf("Error: %v", err)
			failures++
		}

		completed++
	}

	printResults(&resAgg)
	printMetrics(b.metrics)
	if failures > 0 {
		log.Errorf("%d requests failed out of %d", failures, completed)
	}
}

func parseOptions(options *Options) {
	flag.StringVar(&options.host, "u", "localhost:50051", "RPC server host and port")
	flag.IntVar(&options.concurrency, "c", 10, "Number of concurrent requests")
	flag.IntVar(&options.capacity, "cap", 30, "RPC clients pool capacity (max size)")
	flag.IntVar(&options.total, "t", 1000, "Total number of requests to perform")
	flag.StringVar(&options.mode, "mode", "perform", "Choose benchmark mode: connect or perform")
	flag.StringVar(&options.channel, "channel", "BenchmarkChannel", "Channel to subscribe to")
	flag.StringVar(&options.action, "action", "echo", "Action to perform")
	flag.BoolVar(&options.debug, "debug", false, "Enable debug logging")
	flag.Parse()
}

func (b *Benchmark) startWorker() {
	env := &common.SessionEnv{URL: "/cable", Headers: &map[string]string{"cookie": "test_id=rpc-bench"}}
	uuid, err := nanoid.Nanoid()

	if err != nil {
		panic("Nanoid failed")
	}

	// perform authenticate to warmup "session"
	res, err := b.rpc.Authenticate(uuid, env)
	ids := res.Identifier

	if err != nil {
		log.Errorf("Failed to authenticate: %v", err)
		return
	}

	for {
		_, err := b.buffer.Get()

		if err != nil {
			panic(fmt.Errorf("Failed to read from buffer: %v", err))
		}

		start := time.Now()

		if b.perform {
			_, err = b.rpc.Perform(uuid, env, ids, b.channel, b.action)
		} else {
			_, err = b.rpc.Authenticate(uuid, env)
		}

		if err != nil {
			b.errChan <- err
		} else {
			b.resChan <- time.Since(start)
		}
	}
}

func printResults(res *stats.ResAggregate) {
	log.Infof("95p=%dms    min=%dms    median=%dms    max=%dms",
		stats.RoundToMS(res.Percentile(95)),
		stats.RoundToMS(res.Min()),
		stats.RoundToMS(res.Percentile(50)),
		stats.RoundToMS(res.Max()),
	)
}

func printMetrics(m *metrics.Metrics) {
	stats := m.IntervalSnapshot()

	if stats["rpc_retries_total"] > 0 {
		log.Warnf("%d retries were made", stats["rpc_retries_total"])
	}
}
