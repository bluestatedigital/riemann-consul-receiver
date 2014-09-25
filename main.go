package main

import (
    "os"
    "fmt"
    "time"
    
    log "github.com/Sirupsen/logrus"
    flags "github.com/jessevdk/go-flags"

    "github.com/armon/consul-api"
    "github.com/amir/raidman"
)

type Options struct {
    Debug          bool   `          long:"debug"                                            description:"enable debug logging"`
    LogFile        string `short:"l" long:"log-file"                                         description:"JSON log file path"`
    RiemannHost    string `          long:"riemann-host" required:"true"                     description:"Riemann host"`
    RiemannPort    int    `          long:"riemann-port"                 default:"5555"      description:"Riemann port"`
    Proto          string `          long:"proto"                        default:"udp"       description:"protocol to use when sending Riemann events"`
    ConsulHost     string `          long:"consul-host"                  default:"127.0.0.1" description:"Consul host"`
    ConsulPort     int    `          long:"consul-port"                  default:"8500"      description:"Consul port"`
    UpdateInterval string `          long:"interval"                     default:"1m"        description:"how frequently to post events to Riemann"`
}

func main() {
    var opts Options
    // var riemann RiemannClient
    
    _, err := flags.Parse(&opts)
    if err != nil {
        os.Exit(1)
    }
    
    // parse UpdateInterval before setting up logging
    updateInterval, err := time.ParseDuration(opts.UpdateInterval)
    checkError(fmt.Sprintf("invalid update interval %s", opts.UpdateInterval), err)
    
    if opts.Debug {
        // Only log the warning severity or above.
        log.SetLevel(log.DebugLevel)
    }
    
    if opts.LogFile != "" {
        logFp, err := os.OpenFile(opts.LogFile, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0600)
        checkError(fmt.Sprintf("error opening %s", opts.LogFile), err)
        
        defer logFp.Close()
        
        // log as JSON
        log.SetFormatter(&log.JSONFormatter{})
        
        // send output to file
        log.SetOutput(logFp)
    }
    
    // connect to Consul
    consulConfig := consulapi.DefaultConfig()
    consulConfig.Address = fmt.Sprintf("%s:%d", opts.ConsulHost, opts.ConsulPort)
    log.Infof("connecting to Consul at %s", consulConfig.Address)

    consul, err := consulapi.NewClient(consulConfig)
    checkError("unable to create consul client", err)
    
    receiver, err := NewConsulReceiver(
        consul,
        updateInterval,
        "riemann-consul-receiver",
        "services/riemann-consul-receiver",
    )
    
    checkError("unable to initialize consul receiver", err)
    
    err = receiver.RegisterService()
    checkError("unable to register service", err)
    
    _, err = receiver.InitSession()
    checkError("unable to init session", err)
    
    defer receiver.DestroySession()
    
    // connect to Riemann
    riemannAddr := fmt.Sprintf("%s:%d", opts.RiemannHost, opts.RiemannPort)
    log.Infof("connecting to Riemann at %s", riemannAddr)

    riemann, err := raidman.Dial(opts.Proto, riemannAddr)
    if err != nil { // might die
        log.Fatalf("unable to create riemann client: %v", err)
    }
    
    for {
        err := receiver.UpdateHealthCheck()
        checkError("unable to submit health check", err)

        haveLock, err := receiver.AcquireLock(updateInterval)
        checkError("error acquiring lock", err)

        if haveLock {
            healthResults, err := receiver.GetHealthResults(updateInterval)
            
            if err != nil {
                log.Errorf("unable to retrieve health results: %v", err)
                
                receiver.ReleaseLock()
            } else {
                for _, healthCheck := range healthResults {
                    // {
                    //   "Name": "Service 'client-youngaustria' check",
                    //   "ServiceName": "client-youngaustria",
                    //   "Node": "web-fwork-gen-028.us-east-1.aws.prod.bsdinternal.com"
                    //   "CheckID": "service:client-youngaustria",
                    //   "ServiceID": "client-youngaustria",
                    //   "Output": "TTL expired",
                    //   "Notes": "",
                    //   "Status": "critical",
                    // },

                    // Riemann event TTL: A floating-point time, in seconds, that
                    // this event is considered valid for
                    eventTtl := float32((updateInterval * 3) / time.Second)
                    
                    // convert Consul status to Riemann state
                    state := map[string]string{
                        "passing":  "ok",
                        "warning":  "warning",
                        "critical": "critical",
                    }[healthCheck.Status]
                    
                    evt := &raidman.Event{
                        Ttl:         eventTtl,
                        Time:        time.Now().Unix(),
                        Tags:        append([]string{ "consul" }),
                        Host:        healthCheck.Node,
                        State:       state,
                        Service:     healthCheck.CheckID, // @todo CheckID or ServiceID?
                        Description: healthCheck.Output,
                    }
                    
                    err := riemann.Send(evt)
                    
                    if err != nil {
                        log.Errorf("error sending event to Riemann: %v", err)
                        
                        receiver.ReleaseLock()
                        break
                    }
                }
            }
        }
    }
}
