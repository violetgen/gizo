package p2p

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Lobarr/lane"
	upnp "github.com/NebulousLabs/go-upnp"

	"github.com/gizo-network/gizo/core/difficulty"
	"github.com/gizo-network/gizo/core/merkletree"

	"github.com/gizo-network/gizo/job"

	"github.com/gizo-network/gizo/helpers"
	"github.com/gizo-network/gizo/job/queue"
	melody "gopkg.in/olahol/melody.v1"

	"github.com/boltdb/bolt"

	"github.com/kpango/glg"

	"github.com/gizo-network/gizo/benchmark"
	"github.com/gizo-network/gizo/cache"
	"github.com/gizo-network/gizo/core"
	"github.com/gizo-network/gizo/crypt"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/websocket"
)

var (
	ErrJobsFull = errors.New("Jobs array full")
)

type Dispatcher struct {
	IP        string
	Port      uint   //port
	Pub       []byte //public key of the node
	priv      []byte //private key of the node
	uptime    int64  //time since node has been up
	jobPQ     *queue.JobPriorityQueue
	workers   map[*melody.Session]*WorkerInfo //worker nodes in dispatcher's area
	neighbors map[interface{}]*DispatcherInfo
	workerPQ  *WorkerPriorityQueue
	bench     benchmark.Engine //benchmark of node
	wWS       *melody.Melody   //workers ws server
	dWS       *melody.Melody   //dispatchers ws server
	rpc       *rpc.Server
	router    *mux.Router
	jc        *cache.JobCache  //job cache
	bc        *core.BlockChain //blockchain
	db        *bolt.DB         //holds topology table
	mu        *sync.Mutex
	jobs      []job.Job // holds done jobs and new jobs submitted to the network before being placed in the bc
	interrupt chan os.Signal
	writeQ    *lane.Queue
	centrum   *Centrum
	discover  *upnp.IGD
	new       bool // if true, sends a new dispatcher to centrum else sends a wake with token
}

func (d Dispatcher) GetJobs() []job.Job {
	return d.jobs
}

func (d Dispatcher) watchWriteQ() {
	for {
		if d.GetWriteQ().Empty() == false {
			jobs := d.GetWriteQ().Dequeue()
			d.WriteJobs(jobs.([]job.Job))
		}
	}
}

//WriteJobs writes jobs to the bc
func (d Dispatcher) WriteJobs(jobs []job.Job) {
	nodes := []*merkletree.MerkleNode{}
	for _, job := range jobs {
		nodes = append(nodes, merkletree.NewNode(job, &merkletree.MerkleNode{}, &merkletree.MerkleNode{}))
	}
	block := core.NewBlock(*merkletree.NewMerkleTree(nodes), d.GetBC().GetLatestBlock().GetHeader().GetHash(), d.GetBC().GetNextHeight(), uint8(difficulty.Difficulty(d.GetBenchmarks(), *d.GetBC())), d.GetPubString())
	d.GetBC().AddBlock(block)
}

func (d *Dispatcher) AddJob(j job.Job) {
	if len(d.GetJobs()) < merkletree.MaxTreeJobs {
		for i, val := range d.GetJobs() {
			if val.GetID() == j.GetID() {
				temp := val
				temp.AddExec(j.GetLatestExec())
				d.jobs[i] = temp
				return
			}
		}
		d.jobs = append(d.jobs, j)
	} else {
		d.GetWriteQ().Enqueue(d.GetJobs())
		d.EmptyJobs()
		d.jobs = append(d.jobs, j)
	}
}

func (d *Dispatcher) EmptyJobs() {
	d.jobs = []job.Job{}
}

func (d Dispatcher) GetWorkerPQ() *WorkerPriorityQueue {
	return d.workerPQ
}

func (d Dispatcher) GetAssignedWorker(hash []byte) *melody.Session {
	for key, val := range d.GetWorkers() {
		if bytes.Compare(val.GetJob().GetJob().GetHash(), hash) == 0 {
			return key
		}
	}
	return nil
}

func (d Dispatcher) NodeTypeDispatcher() bool {
	return true
}

func (d Dispatcher) GetIP() string {
	return d.IP
}

func (d *Dispatcher) SetIP(ip string) {
	d.IP = ip
}

func (d Dispatcher) GetPubByte() []byte {
	return d.Pub
}

func (d Dispatcher) GetPubString() string {
	return hex.EncodeToString(d.Pub)
}

func (d Dispatcher) GetPrivByte() []byte {
	return d.priv
}

func (d Dispatcher) GetPrivString() string {
	return hex.EncodeToString(d.priv)
}

func (d Dispatcher) GetWriteQ() *lane.Queue {
	return d.writeQ
}

func (d Dispatcher) GetJobPQ() *queue.JobPriorityQueue {
	return d.jobPQ
}

func (d Dispatcher) GetWorkers() map[*melody.Session]*WorkerInfo {
	return d.workers
}

func (d Dispatcher) GetWorker(s *melody.Session) *WorkerInfo {
	return d.GetWorkers()[s]
}

func (d *Dispatcher) SetWorker(s *melody.Session, w *WorkerInfo) {
	d.GetWorkers()[s] = w
}

func (d Dispatcher) GetBC() *core.BlockChain {
	return d.bc
}

func (d Dispatcher) GetPort() int {
	return int(d.Port)
}

func (d Dispatcher) GetUptme() int64 {
	return d.uptime
}

func (d Dispatcher) GetUptimeString() string {
	return time.Unix(d.uptime, 0).Sub(time.Now()).String()
}

func (d Dispatcher) GetBench() benchmark.Engine {
	return d.bench
}

func (d Dispatcher) GetBenchmarks() []benchmark.Benchmark {
	return d.bench.GetData()
}

func (d Dispatcher) GetJC() *cache.JobCache {
	return d.jc
}

func (d Dispatcher) GetWWS() *melody.Melody {
	return d.wWS
}

func (d *Dispatcher) setWWS(m *melody.Melody) {
	d.wWS = m
}

func (d Dispatcher) GetDWS() *melody.Melody {
	return d.dWS
}

func (d *Dispatcher) setDWS(m *melody.Melody) {
	d.dWS = m
}

func (d Dispatcher) GetRPC() *rpc.Server {
	return d.rpc
}

func (d Dispatcher) setRPC(s *rpc.Server) {
	d.rpc = s
}

func (d Dispatcher) BroadcastWorkers(m []byte) {
	for s, _ := range d.workers {
		s.Write(m)
	}
}

func (d Dispatcher) WorkerExists(s *melody.Session) bool {
	_, ok := d.workers[s]
	return ok
}

func (d Dispatcher) deployJobs() {
	for {
		if d.GetWorkerPQ().getPQ().Empty() == false {
			if d.GetJobPQ().GetPQ().Empty() == false {
				d.mu.Lock()
				w := d.GetWorkerPQ().Pop()
				if !d.GetWorker(w).GetShut() {
					j := d.GetJobPQ().Pop()
					if j.GetExec().GetStatus() != job.CANCELLED {
						j.GetExec().SetBy(d.GetWorker(w).GetPub())
						d.GetWorker(w).Assign(&j)
						glg.Info("P2P: dispatched job")
						w.Write(JobMessage(j.Serialize(), d.GetPrivByte()))
					} else {
						j.ResultsChan() <- j
					}
				} else {
					delete(d.GetWorkers(), w)
				}
				d.mu.Unlock()
			}
		}
	}
}

func (d Dispatcher) wPeerTalk() {
	d.wWS.HandleDisconnect(func(s *melody.Session) {
		d.mu.Lock()
		glg.Info("Dispatcher: worker disconnected")
		if d.GetWorker(s).GetJob() != nil {
			d.GetJobPQ().PushItem(*d.GetWorker(s).GetJob(), job.HIGH)
		}
		d.GetWorker(s).SetShut(true)
		d.mu.Unlock()
	})
	d.wWS.HandleMessageBinary(func(s *melody.Session, message []byte) {
		m := DeserializePeerMessage(message)
		switch m.GetMessage() {
		case HELLO:
			d.mu.Lock()
			if len(d.GetWorkers()) < MaxWorkers {
				glg.Info("Dispatcher: worker connected")
				d.SetWorker(s, NewWorkerInfo(hex.EncodeToString(m.GetPayload())))
				s.Write(HelloMessage(d.GetPubByte()))
				d.centrum.ConnectWorker()
				d.GetWorkerPQ().Push(s, 0)
			} else {
				s.Write(ConnFullMessage())
			}
			d.mu.Unlock()
			break
		case RESULT:
			d.mu.Lock()
			if m.VerifySignature(d.GetWorker(s).GetPub()) {
				glg.Info("P2P: received result")
				exec := job.DeserializeExec(m.GetPayload())
				d.GetWorker(s).GetJob().SetExec(&exec)
				d.GetWorker(s).GetJob().ResultsChan() <- *d.GetWorker(s).GetJob()
				j := d.GetWorker(s).GetJob().GetJob()
				d.GetWorker(s).SetJob(nil)
				j.AddExec(exec)
				d.AddJob(j)
			} else {
				d.GetJobPQ().PushItem(*d.GetWorker(s).GetJob(), job.HIGH)
			}
			if !d.GetWorker(s).GetShut() {
				d.GetWorkerPQ().Push(s, 0)
			}
			d.mu.Unlock()
			break
		case SHUT:
			d.mu.Lock()
			d.GetWorker(s).SetShut(true)
			s.Write(ShutAckMessage(d.GetPrivByte()))
			d.centrum.DisconnectWorker()
			d.mu.Unlock()
			break
		default:
			s.Write(InvalidMessage())
			break
		}
	})
}

func (d Dispatcher) dPeerTalk() {

}

func (d Dispatcher) WatchInterrupt() {
	select {
	case i := <-d.interrupt:
		glg.Warn("Dispatcher: interrupt detected")
		switch i {
		case syscall.SIGINT, syscall.SIGTERM:
			res, err := d.centrum.Sleep()
			if err != nil {
				glg.Fatal(err)
			} else if res["status"].(string) != "success" {
				glg.Fatal("Centrum: " + res["status"].(string))
			}
			d.BroadcastWorkers(ShutMessage(d.GetPrivByte()))
			time.Sleep(time.Second * 3) // give neighbors and workers 3 seconds to disconnect
			os.Exit(0)
		case syscall.SIGQUIT:
			os.Exit(1)
		}
	}
}

func (d Dispatcher) Start() {
	if !d.GetBC().Verify() {
		glg.Fatal("Dispatcher: blockchain not verified")
	}
	go d.deployJobs()
	go d.watchWriteQ()
	go d.WatchInterrupt()
	d.wWS.Upgrader.ReadBufferSize = 100000
	d.wWS.Upgrader.WriteBufferSize = 100000
	d.wWS.Config.MessageBufferSize = 100000
	d.wWS.Config.MaxMessageSize = 100000
	d.wWS.Upgrader.EnableCompression = true
	d.dWS.Upgrader.ReadBufferSize = 100000
	d.dWS.Upgrader.WriteBufferSize = 100000
	d.dWS.Config.MessageBufferSize = 100000
	d.dWS.Config.MaxMessageSize = 100000
	d.dWS.Upgrader.EnableCompression = true
	d.router.HandleFunc("/d", func(w http.ResponseWriter, r *http.Request) {
		d.dWS.HandleRequest(w, r)
	})
	d.router.HandleFunc("/w", func(w http.ResponseWriter, r *http.Request) {
		d.wWS.HandleRequest(w, r)
	})
	d.wPeerTalk()
	d.router.Handle("/rpc", d.rpc).Methods("POST")
	status := make(map[string]interface{})
	status["status"] = "running"
	status["pub"] = d.GetPubString()
	statusBytes, err := json.Marshal(status)
	if err != nil {
		glg.Fatal(err)
	}
	d.router.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write(statusBytes)
	})

	err = d.discover.Forward(uint16(d.GetPort()), "gizo dispatcher node")
	if err != nil {
		log.Fatal(err)
	}
	if d.new {
		go d.Register()
	} else {
		res, err := d.centrum.Wake()
		if err != nil {
			glg.Fatal(err)
		} else if res["status"].(string) != "success" {
			glg.Fatal("Centrum: " + res["status"].(string))
		}
	}

	//! TLS config
	// certManager := autocert.Manager{
	// 	Prompt: autocert.AcceptTOS,
	// 	Cache:  autocert.DirCache("ssl"),
	// }

	// server := &http.Server{
	// 	Handler: d.router,
	// 	Addr:    ":8443",
	// 	TLSConfig: &tls.Config{
	// 		GetCertificate: certManager.GetCertificate,
	// 	},
	// }

	// err = d.discover.Forward(8443, "gizo dispatcher node")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// go http.ListenAndServe(":"+strconv.FormatInt(int64(d.GetPort()), 10), d.router)
	// fmt.Println(server.ListenAndServeTLS("", ""))
	fmt.Println(http.ListenAndServe(":"+strconv.FormatInt(int64(d.GetPort()), 10), d.router))
}

func (d Dispatcher) SaveToken() {
	err := d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(NodeBucket))
		if err := b.Put([]byte("token"), []byte(d.centrum.GetToken())); err != nil {
			glg.Fatal(err)
		}
		return nil
	})
	if err != nil {
		glg.Fatal(err)
	}
}

func (d Dispatcher) Register() {
	time.Sleep(time.Second * 1)
	if err := d.centrum.NewDisptcher(d.GetPubString(), d.GetIP(), int(d.GetPort())); err != nil {
		glg.Warn("Centrum: unable to get on network")
		glg.Fatal("Centrum: " + err.Error())
	}
	d.SaveToken()
}

func (d *Dispatcher) GetDispatchers() {
	res := d.centrum.GetDispatchers()
	dispatchers, ok := res["dispatchers"]
	if !ok {
		glg.Warn(ErrNoDispatchers)
		return
	}
	for _, dispatcher := range dispatchers.([]string) {
		addr, err := ParseAddr(dispatcher)
		if err == nil && addr["pub"].(string) != d.GetPubString() {
			url := fmt.Sprintf("ws://%v:%v/w", addr["ip"], addr["port"])
			dailer := websocket.Dialer{
				Proxy:           http.ProxyFromEnvironment,
				ReadBufferSize:  10000,
				WriteBufferSize: 10000,
			}
			conn, _, err := dailer.Dial(url, nil)
			if err != nil {
				glg.Fatal(err)
			}
			conn.EnableWriteCompression(true)
			d.neighbors[conn] = NewDispatcherInfo(addr["pub"].(string))
			fmt.Println("added connection")
		}
	}
}

func NewDispatcher(port int) *Dispatcher {
	glg.Info("Creating Dispatcher Node")
	core.InitializeDataPath()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	var bench benchmark.Engine
	var priv, pub []byte
	var token string
	discover, err := upnp.Discover()
	if err != nil {
		glg.Fatal(err)
	}

	ip, err := discover.ExternalIP()
	if err != nil {
		log.Fatal(err)
	}

	centrum := NewCentrum()

	var dbFile string
	if os.Getenv("ENV") == "dev" {
		dbFile = path.Join(core.IndexPathDev, NodeDB)
	} else {
		dbFile = path.Join(core.IndexPathProd, NodeDB)
	}

	if helpers.FileExists(dbFile) {
		glg.Warn("Dispatcher: using existing keypair and benchmark")
		db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: time.Second * 2})
		if err != nil {
			glg.Fatal(err)
		}
		err = db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(NodeBucket))
			priv = b.Get([]byte("priv"))
			pub = b.Get([]byte("pub"))
			bench = benchmark.DeserializeBenchmarkEngine(b.Get([]byte("benchmark")))
			token = string(b.Get([]byte("token")))
			return nil
		})
		if err != nil {
			glg.Fatal(err)
		}
		centrum.SetToken(token)
		bc := core.CreateBlockChain(hex.EncodeToString(pub))
		jc := cache.NewJobCache(bc)
		return &Dispatcher{
			IP:        ip,
			Pub:       pub,
			priv:      priv,
			Port:      uint(port),
			uptime:    time.Now().Unix(),
			bench:     bench,
			jobPQ:     queue.NewJobPriorityQueue(),
			workers:   make(map[*melody.Session]*WorkerInfo),
			workerPQ:  NewWorkerPriorityQueue(),
			jc:        jc,
			bc:        bc,
			db:        db,
			router:    mux.NewRouter(),
			wWS:       melody.New(),
			dWS:       melody.New(),
			rpc:       rpc.NewServer(),
			mu:        new(sync.Mutex),
			interrupt: interrupt,
			writeQ:    lane.NewQueue(),
			centrum:   centrum,
			discover:  discover,
			new:       false,
		}
	}

	priv, pub = crypt.GenKeys()
	bench = benchmark.NewEngine()
	db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: time.Second * 2})
	if err != nil {
		glg.Fatal(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucket([]byte(NodeBucket))
		if err != nil {
			glg.Fatal(err)
		}

		if err = b.Put([]byte("benchmark"), bench.Serialize()); err != nil {
			glg.Fatal(err)
		}

		if err = b.Put([]byte("priv"), priv); err != nil {
			glg.Fatal(err)
		}

		if err = b.Put([]byte("pub"), pub); err != nil {
			glg.Fatal(err)
		}
		return nil
	})

	if err != nil {
		glg.Fatal(err)
	}
	bc := core.CreateBlockChain(hex.EncodeToString(pub))
	jc := cache.NewJobCache(bc)
	return &Dispatcher{
		IP:        ip,
		Pub:       pub,
		priv:      priv,
		Port:      uint(port),
		uptime:    time.Now().Unix(),
		bench:     bench,
		jobPQ:     queue.NewJobPriorityQueue(),
		workers:   make(map[*melody.Session]*WorkerInfo),
		workerPQ:  NewWorkerPriorityQueue(),
		jc:        jc,
		bc:        bc,
		db:        db,
		router:    mux.NewRouter(),
		wWS:       melody.New(),
		dWS:       melody.New(),
		rpc:       rpc.NewServer(),
		mu:        new(sync.Mutex),
		interrupt: interrupt,
		writeQ:    lane.NewQueue(),
		centrum:   centrum,
		discover:  discover,
		new:       true,
	}
}
