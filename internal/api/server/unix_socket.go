package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sync"

	"github.com/edulinq/autograder/internal/api/core"
	"github.com/edulinq/autograder/internal/config"
	"github.com/edulinq/autograder/internal/lockmanager"
	"github.com/edulinq/autograder/internal/log"
	"github.com/edulinq/autograder/internal/systemserver"
	"github.com/edulinq/autograder/internal/util"
)

const (
	// Base path for the API endpoint.
	ENDPOINT_KEY                 = "endpoint"
	NONCE_KEY                    = "root-user-nonce"
	NONCE_SIZE_BYTES             = 64
	REQUEST_KEY                  = "request"
	UNIX_SOCKET_SERVER_STOP_LOCK = "internal.api.server.UNIX_SOCKET_STOP_LOCK"
)

var unixSocket net.Listener

func runUnixSocketServer(subserverSetupWaitGroup *sync.WaitGroup) (err error) {
	defer func() {
		value := recover()
		if value == nil {
			return
		}

		err = errors.Join(err, fmt.Errorf("Unix socket panicked: '%v'.", value))
	}()

	err = createSocket()
	subserverSetupWaitGroup.Done()
	if err != nil {
		return fmt.Errorf("Failed to create UNIX socket: '%w'.", err)
	}

	defer stopUnixSocketServer()

	for {
		connection, err := unixSocket.Accept()
		if err != nil {
			log.Info("Unix Socket Server Stopped.")

			if unixSocket == nil {
				// Set err to nil if the unix socket gracefully closed.
				err = nil
			}

			if err != nil {
				log.Error("Unix socket server returned an error.", err)
			}

			return err
		}

		go func() {
			defer func() {
				err := connection.Close()
				if err != nil {
					log.Error("Failed to close the connection.", err)
				}
			}()

			err = handleUnixSocketConnection(connection)
			if err != nil {
				log.Error("Error handling the unix socket connection.", err)
			}
		}()
	}
}

func createSocket() error {
	unixSocketPath, err := systemserver.GetUnixSocketPath()
	if err != nil {
		return err
	}

	err = util.MkDir(filepath.Dir(unixSocketPath))
	if err != nil {
		return fmt.Errorf("Failed to create the unix socket: '%w'.", err)
	}

	unixSocket, err = net.Listen("unix", unixSocketPath)
	if err != nil {
		return fmt.Errorf("Failed to listen on the unix socket: '%w'.", err)
	}

	log.Info("Unix Socket Server Started.", log.NewAttr("unix_socket", unixSocketPath))

	return nil
}

// Read the request from the unix socket, generate and store a nonce, add the nonce to the request,
// send the request to the API endpoint, and write the response back to the unix socket.
func handleUnixSocketConnection(connection net.Conn) error {
	var port = config.WEB_HTTP_PORT.Get()

	jsonBuffer, err := util.ReadFromNetworkConnection(connection)
	if err != nil {
		return fmt.Errorf("Failed to read from the unix socket: '%w'.", err)
	}

	randomNonce, err := util.RandHex(NONCE_SIZE_BYTES)
	if err != nil {
		return fmt.Errorf("Failed to generate the nonce: '%w'.", err)
	}

	core.RootUserNonces.Store(randomNonce, true)
	defer core.RootUserNonces.Delete(randomNonce)

	var payload map[string]any
	err = json.Unmarshal(jsonBuffer, &payload)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal the request buffer into the payload: '%w'.", err)
	}

	basePath, exists := payload[ENDPOINT_KEY].(string)
	if !exists {
		return fmt.Errorf("Failed to find the 'endpoint' key in the request.")
	}

	fullAPIPath := core.MakeFullAPIPath(basePath)

	content, exists := payload[REQUEST_KEY].(map[string]any)
	if !exists {
		return fmt.Errorf("Failed to find the 'request' key in the request.")
	}

	content[NONCE_KEY] = randomNonce
	formContent, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("Failed to marshal the request's content: '%w'.", err)
	}

	form := make(map[string]string)
	form[core.API_REQUEST_CONTENT_KEY] = string(formContent)

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, fullAPIPath)
	responseText, err := util.PostNoCheck(url, form)
	if err != nil {
		return fmt.Errorf("Failed to POST an API request: '%w'.", err)
	}

	jsonResponseBytes := []byte(responseText)

	err = util.WriteToNetworkConnection(connection, jsonResponseBytes)
	if err != nil {
		return fmt.Errorf("Failed to write to the unix socket: '%w'.", err)
	}

	return nil
}

func stopUnixSocketServer() {
	lockmanager.Lock(UNIX_SOCKET_SERVER_STOP_LOCK)
	defer lockmanager.Unlock(UNIX_SOCKET_SERVER_STOP_LOCK)

	if unixSocket == nil {
		return
	}

	tempUnixSocket := unixSocket
	unixSocket = nil

	err := tempUnixSocket.Close()
	if err != nil {
		log.Error("Failed to close the unix socket.", err)
	}
}
