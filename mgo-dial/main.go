package main

// Debug mgo.Dial
// https://github.com/daniel-nichter/lab

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/globalsign/mgo"
)

var (
	flagUsername    string
	flagPassword    string
	flagSource      string
	flagService     string
	flagServiceHost string
	flagMechanism   string

	flagTLSCert string
	flagTLSKey  string
	flagTLSCA   string

	flagTimeout  uint
	flagDebug    bool
	flagIsMaster bool
	flagLogin    bool
)

func init() {
	flag.StringVar(&flagUsername, "username", "", "Credential.Username")
	flag.StringVar(&flagPassword, "password", "", "Credential.Password")
	flag.StringVar(&flagSource, "source", "", "Credential.Source")
	flag.StringVar(&flagService, "service", "", "Credential.Service")
	flag.StringVar(&flagServiceHost, "service-host", "", "Credential.ServiceHost")
	flag.StringVar(&flagMechanism, "mechanism", "", "Credential.Mechanism")

	flag.StringVar(&flagTLSCert, "tls-cert", "", "TLS certificate file")
	flag.StringVar(&flagTLSKey, "tls-key", "", "TLS key file")
	flag.StringVar(&flagTLSCA, "tls-ca", "", "TLS certificate authority")

	flag.UintVar(&flagTimeout, "timeout", 3000, "Dial timeout (milliseconds)")
	flag.BoolVar(&flagDebug, "debug", false, "Enable mgo debug to STDERR")
	flag.BoolVar(&flagIsMaster, "ismaster", false, "Print partial isMaster result after login")
	flag.BoolVar(&flagLogin, "login", true, "Session.Login() with credentials")
}

type Node struct {
	Host           string            `bson:"me"`
	ReplSetName    string            `bson:"setName"`
	ReplSetVersion uint              `bson:"setVersion"`
	PrimaryHost    string            `bson:"primary"`
	IsMatser       bool              `bson:"ismaster"`
	Secondary      bool              `bson:"secondary"`
	Tags           map[string]string `bson:"tags"`
}

func main() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds)
	log.SetOutput(os.Stdout)

	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Printf("Usage: mgo-dial [flags] URL\n")
		os.Exit(1)
	}

	if flagDebug {
		dbg := log.New(os.Stderr, "DEBUG ", log.Lshortfile|log.Ldate|log.Lmicroseconds)
		mgo.SetLogger(dbg)
		mgo.SetDebug(true)
	}

	url := args[0]
	fmt.Printf("url: %s\n", url)

	// Load TLS if given
	var tlsConfig *tls.Config
	if (flagTLSCert != "" && flagTLSKey != "") || flagTLSCA != "" {
		tlsConfig = &tls.Config{}

		if flagTLSCA != "" {
			caCert, err := ioutil.ReadFile(flagTLSCA)
			if err != nil {
				log.Fatal(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.RootCAs = caCertPool
			fmt.Println("TLS root CA loaded")
		}

		if flagTLSCert != "" && flagTLSKey != "" {
			cert, err := tls.LoadX509KeyPair(flagTLSCert, flagTLSKey)
			if err != nil {
				log.Fatal(err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
			tlsConfig.BuildNameToCertificate()
			fmt.Println("TLS cert/key loaded")
		}
	} else {
		fmt.Println("TLS cert and key not given")
	}

	// Make custom dialer that can do TLS
	dialInfo, err := mgo.ParseURL(url)
	if err != nil {
		log.Fatalf("mgo.ParseURL: %s", err)
	}
	dialInfo.Username = flagUsername
	dialInfo.Password = flagPassword
	dialInfo.Source = flagSource
	dialInfo.Mechanism = flagMechanism
	dialInfo.Service = flagService
	dialInfo.ServiceHost = flagServiceHost
	timeout := time.Duration(flagTimeout) * time.Millisecond
	fmt.Printf("timeout: %s\n", timeout)
	dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		if tlsConfig != nil {
			dialer := &net.Dialer{
				Timeout: timeout,
			}
			fmt.Printf("dialing (TLS) %s...\n", addr.String())
			conn, err := tls.DialWithDialer(dialer, "tcp", addr.String(), tlsConfig)
			if err != nil {
				log.Println(err)
			}
			return conn, err
		} else {
			fmt.Printf("dialing %s...\n", addr.String())
			conn, err := net.DialTimeout("tcp", addr.String(), timeout)
			if err != nil {
				log.Println(err)
			}
			return conn, err
		}
	}

	// Connect
	s, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.Fatalf("mgo.DialWithInfo: %s", err)
	}
	fmt.Println("connected")

	// Login
	if flagLogin {
		cred := &mgo.Credential{
			Username:    flagUsername,
			Password:    flagPassword,
			Source:      flagSource,
			Service:     flagService,
			ServiceHost: flagServiceHost,
			Mechanism:   flagMechanism,
		}
		log.Printf("mgo.Credential: %#v", *cred)
		if err := s.Login(cred); err != nil {
			log.Printf("mgo.Session.Login: %s", err)
		} else {
			fmt.Println("login successful")
		}
	}

	if flagIsMaster {
		var node Node
		if err := s.Run("isMaster", &node); err != nil {
			log.Fatalf("isMaster: %s", err)
		}
		fmt.Printf("isMaster: %#v\n", node)
	}

	fmt.Println("SUCCESS")
}
