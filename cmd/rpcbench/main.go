package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/golang-collections/go-datastructures/queue"

	grpcpool "github.com/anycable/anycable-go/pool"
	pb "github.com/anycable/anycable-go/protos"
	"github.com/anycable/anycable-go/utils"
	"github.com/apex/log"

	"github.com/namsral/flag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	retryInterval = 10
	invokeTimeout = 3000
)

// Options describes benchmark options
type Options struct {
	host            string
	capacity        int
	initialCapacity int
	concurrency     int
	total           int
	pool            bool
	debug           bool
}

// Benchmark implements shared methods for the benchmark
type Benchmark struct {
	buffer  *queue.RingBuffer
	pool    *grpcpool.Pool
	sem     chan (struct{})
	conn    *grpc.ClientConn
	resChan chan (time.Duration)
	errChan chan (error)
}

var kacp = keepalive.ClientParameters{
	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	PermitWithoutStream: true,             // send pings even without active streams
}

var retryCount = uint64(0)
var retryTime = uint64(0)

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
		"pool",
		options.pool,
	).Infof("Running RPC benchmark for %s", options.host)

	factory := func() (*grpc.ClientConn, error) {
		return grpc.Dial(options.host, grpc.WithInsecure(), grpc.WithKeepaliveParams(kacp))
	}

	if options.pool {
		p, err := grpcpool.NewChannelPool(options.initialCapacity, options.capacity, factory)

		if err != nil {
			log.Errorf("!!! Failed to intialize RPC pool !!!\n%v", err)
			os.Exit(1)
		}

		b.pool = &p
	} else {
		conn, err := grpc.Dial(options.host, grpc.WithInsecure(), grpc.WithKeepaliveParams(kacp), grpc.WithBalancerName("round_robin"))

		if err != nil {
			log.Errorf("!!! Failed to intialize RPC connection !!!\n%v", err)
			os.Exit(1)
		}

		b.sem = make(chan struct{}, options.capacity)
		for i := 0; i < options.capacity; i++ {
			b.sem <- struct{}{}
		}
		b.conn = conn
	}

	b.resChan = make(chan time.Duration, 1000)
	b.errChan = make(chan error, 1000)
	b.buffer = queue.NewRingBuffer(uint64(options.total))

	for i := 0; i < options.total; i++ {
		b.buffer.Put(nil)
	}

	for i := 0; i < options.concurrency; i++ {
		go b.startWorker()
	}

	var resAgg resAggregate

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
	if failures > 0 {
		log.Errorf("%d requests failed out of %d", failures, completed)
	}
	if retryCount > 0 {
		log.Warnf("%d requests retried (total time %dms)", retryCount, retryTime)
	}
}

func parseOptions(options *Options) {
	flag.StringVar(&options.host, "u", "localhost:50051", "RPC server host and port")
	flag.IntVar(&options.concurrency, "c", 10, "Number of concurrent requests")
	flag.IntVar(&options.capacity, "cap", 30, "RPC clients pool capacity (max size)")
	flag.IntVar(&options.initialCapacity, "init", 10, "RPC clients pool initial capacity")
	flag.IntVar(&options.total, "t", 1000, "Total number of requests to perform")
	flag.BoolVar(&options.pool, "pool", false, "Use clients pool or single client")
	flag.BoolVar(&options.debug, "debug", false, "Enable debug logging")
	flag.Parse()
}

func (b *Benchmark) startWorker() {
	for {
		_, err := b.buffer.Get()

		if err != nil {
			panic(fmt.Errorf("Failed to read from buffer: %v", err))
		}

		start := time.Now()
		if b.pool != nil {
			err = b.performPoolRequest()
		} else {
			err = b.performRequest()
		}

		if err != nil {
			b.errChan <- err
		} else {
			b.resChan <- time.Now().Sub(start)
		}
	}
}

func (b *Benchmark) performPoolRequest() (err error) {
	conn, err := (*b.pool).Get()

	if err != nil {
		return
	}

	defer conn.Close()

	client := pb.NewRPCClient(conn.Conn)

	_, err = retry(func() (interface{}, error) {
		return client.Connect(context.Background(), &pb.ConnectionRequest{Path: "/cable", Headers: make(map[string]string)})
	})

	return
}

func (b *Benchmark) performRequest() (err error) {
	<-b.sem
	defer func() { b.sem <- struct{}{} }()

	client := pb.NewRPCClient(b.conn)

	_, err = retry(func() (interface{}, error) {
		return client.Connect(context.Background(), &pb.ConnectionRequest{Path: "/cable", Headers: make(map[string]string)})
	})

	return
}

func retry(callback func() (interface{}, error)) (res interface{}, err error) {
	attempts := int(math.Ceil(math.Log2(invokeTimeout / retryInterval)))

	for i := 0; ; i++ {
		res, err = callback()

		if err == nil {
			return res, nil
		}

		if i >= (attempts - 1) {
			return nil, err
		}

		log.Debugf("RPC failure %v", err)

		delayMS := math.Pow(2, float64(i)) * retryInterval
		delay := time.Duration(delayMS)

		atomic.AddUint64(&retryCount, 1)
		atomic.AddUint64(&retryTime, uint64(delayMS))

		time.Sleep(delay * time.Millisecond)
	}
}

func printResults(res *resAggregate) {
	log.Infof("95p=%dms    min=%dms    median=%dms    max=%dms",
		roundToMS(res.Percentile(95)),
		roundToMS(res.Min()),
		roundToMS(res.Percentile(50)),
		roundToMS(res.Max()),
	)
}

func roundToMS(d time.Duration) int64 {
	return int64((d + (500 * time.Microsecond)) / time.Millisecond)
}

// From https://github.com/anycable/websocket-bench/blob/master/benchmark/stat.go
type resAggregate struct {
	samples []time.Duration
	sorted  bool
}

type byAsc []time.Duration

func (a byAsc) Len() int           { return len(a) }
func (a byAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byAsc) Less(i, j int) bool { return a[i] < a[j] }

func (agg *resAggregate) Add(rtt time.Duration) {
	agg.samples = append(agg.samples, rtt)
	agg.sorted = false
}

func (agg *resAggregate) Count() int {
	return len(agg.samples)
}

func (agg *resAggregate) Min() time.Duration {
	if agg.Count() == 0 {
		return 0
	}
	agg.Sort()
	return agg.samples[0]
}

func (agg *resAggregate) Max() time.Duration {
	if agg.Count() == 0 {
		return 0
	}
	agg.Sort()
	return agg.samples[len(agg.samples)-1]
}

func (agg *resAggregate) Percentile(p int) time.Duration {
	if p <= 0 {
		panic("p must be greater than 0")
	} else if 100 <= p {
		panic("p must be less 100")
	}

	agg.Sort()

	rank := p * len(agg.samples) / 100

	if agg.Count() == 0 {
		return 0
	}

	return agg.samples[rank]
}

func (agg *resAggregate) Sort() {
	if agg.sorted {
		return
	}
	sort.Sort(byAsc(agg.samples))
	agg.sorted = true
}
