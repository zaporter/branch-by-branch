package lambda

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// TODO:
// local: ./scripts/router/push-to-lambda.sh {ip}
//
//	./scripts/lambda-link-cache.sh && ./inference/run_inference.sh
func startInferenceOnLambda(instance string, numTries int, version string) error {
	t := time.NewTicker(8 * time.Second)
	for i := range numTries {
		fmt.Printf("attempt %v to start inference on lambda\n", i)
		status, err := ListInstances()
		if err != nil {
			return err
		}

		for _, c := range status.Data {
			if c.ID == instance {
				if c.IP == nil {
					fmt.Println("Instance still has nil IP")
				} else {
					out, err := startLambdaInference(*c.IP, version)
					if err != nil && i+1 == numTries {
						return err
					} else if err != nil {
						// dont kill because we will try again
						fmt.Println(err)
					} else {
						fmt.Println(out)
						return nil
					}
				}
			}
		}
		<-t.C
	}
	return errors.New("Failed to start inference on lambda instance")
}

func verifyHost(_host string, remote net.Addr, key ssh.PublicKey) error {
	return nil

	//	//
	//	// If you want to connect to new hosts.
	//	// here your should check new connections public keys
	//	// if the key not trusted you shuld return an error
	//	//

	//	// hostFound: is host in known hosts file.
	//	// err: error if key not in known hosts file OR host in known hosts file but key changed!
	//	hostFound, err := goph.CheckKnownHost(host, remote, key, "")

	//	// Host in known hosts but key mismatch!
	//	// Maybe because of MAN IN THE MIDDLE ATTACK!
	//	if hostFound && err != nil {

	//		return err
	//	}

	//	// handshake because public key already exists.
	//	if hostFound && err == nil {

	//		return nil
	//	}

	// fmt.Printf("ADDING %q TO KNOWN_HOSTS", host)
	// // Add the new host to known hosts file.
	// return goph.AddKnownHost(host, remote, key, "")
}

func startLambdaInference(ip string, version string) (string, error) {
	path := fmt.Sprintf("/data/%v/lora-out", version)
	cmd := "/home/ubuntu/proof-gen/etc/start-lambda-inference.sh " + path
	return execOnInstance(ip, cmd)
}

func execOnInstance(ip, command string) (string, error) {
	keyPath := os.Getenv("LAMBDA_KEY_PATH")
	auth, err := goph.Key(keyPath, "")
	if err != nil {
		return "", fmt.Errorf("failed to authenticate with key: %w", err)
	}

	client, err := goph.NewConn(&goph.Config{
		User:     "ubuntu",
		Addr:     ip,
		Port:     22,
		Auth:     auth,
		Timeout:  20 * time.Second,
		Callback: verifyHost,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create client %w", err)
	}

	defer client.Close()

	out, err := client.Run(command)

	if err != nil {
		return "", fmt.Errorf("failed to run command: %w", err)
	}

	return string(out), nil
}
