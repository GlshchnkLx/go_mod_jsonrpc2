package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type ClientTransport interface {
	Execute(request []byte) (response []byte)
}

type ClientTransportHttp struct {
	ClientTransport

	endpoint string
}

func (ct *ClientTransportHttp) Execute(request []byte) (response []byte) {
	var (
		err           error
		requestBuffer *bytes.Buffer
		httpResponse  *http.Response
	)

	defer func() {
		if err != nil {
			responseStruct := &Response{
				Jsonrpc: "2.0",
			}

			if requestBuffer == nil {
				responseStruct.Error = &Error{
					Code:    -32600,
					Message: "Invalid Request",
					Data:    err.Error(),
				}
			} else if httpResponse == nil || response == nil {
				responseStruct.Error = &Error{
					Code:    -32000,
					Message: "Server error",
					Data:    err.Error(),
				}
			} else {
				responseStruct.Error = &Error{
					Code:    -32603,
					Message: "Internal error",
					Data:    err.Error(),
				}
			}

			response, err = json.Marshal(responseStruct)
			if err != nil {
				return
			}
		}
	}()

	if !json.Valid(request) {
		err = errors.New(string(request))
		return
	}
	requestBuffer = bytes.NewBuffer(request)

	httpResponse, err = http.Post(ct.endpoint, "application/json", requestBuffer)
	if err != nil {
		return
	}

	response, err = io.ReadAll(httpResponse.Body)
	if err != nil {
		return
	}

	if !json.Valid(response) {
		err = errors.New(string(response))
		response = nil
		return
	}

	return
}

func NewClientTransportHttp(endpoint string) ClientTransport {
	return &ClientTransportHttp{
		endpoint: endpoint,
	}
}

type Client struct {
	transport ClientTransport

	callId int

	batchMutex        chan interface{}
	batchIsRun        bool
	batchRequestArray []Request
	batchResponseMap  map[int]chan Response
}

func (c *Client) RawRequest(request interface{}) (response interface{}) {
	var (
		err      error
		isBatch  bool
		isSingle bool

		requestSingle  Request
		responceSingle Response
		requestBatch   []Request
		responseBatch  []Response

		requestByte  []byte
		responseByte []byte
	)

	defer func() {
		if err != nil {
			var jsonrpcError Error

			if requestByte == nil || responseByte == nil {
				jsonrpcError = Error{
					Code:    -32603,
					Message: "Internal error",
					Data:    err.Error(),
				}
			}

			if isBatch {
				responseBatch = []Response{}

				for _, requestObject := range requestBatch {
					if requestObject.Id != nil {
						responseBatch = append(responseBatch, Response{
							Jsonrpc: requestObject.Jsonrpc,
							Id:      requestObject.Id,
							Error:   &jsonrpcError,
						})
					}
				}

				response = responseBatch
			} else {
				response = Response{
					Jsonrpc: requestSingle.Jsonrpc,
					Id:      requestSingle.Id,
					Error:   &jsonrpcError,
				}
			}
		}
	}()

	requestBatch, isBatch = request.([]Request)
	requestSingle, isSingle = request.(Request)

	if !(isBatch || isSingle) {
		err = errors.New("Unknown request type")
		return
	}

	requestByte, err = json.Marshal(request)
	if err != nil {
		return
	}

	responseByte = c.transport.Execute(requestByte)

	if isBatch {
		err = json.Unmarshal(responseByte, &responseBatch)
		if err != nil {
			return
		}
		response = responseBatch
	} else {

		err = json.Unmarshal(responseByte, &responceSingle)
		if err != nil {
			return
		}
		response = responceSingle
	}

	return
}

func (c *Client) Request(method string, params, response interface{}, timeout int) (rpcError *Error) {
	var (
		err            error
		ok             bool
		isCall         bool    = response != nil
		jsonrpcRequest Request = Request{
			Jsonrpc: "2.0",
			Method:  method,
		}
		jsonrpcResponse      Response
		jsonrpcArrayResponse []Response
		jsonrpcRawResponse   interface{}
	)

	if isCall {
		jsonrpcRequest.Id = c.callId
		c.callId++
	}

	jsonrpcRequest.Params, err = json.Marshal(params)
	if err != nil {
		return
	}

	if timeout > 0 {
		c.batchMutex <- true
		var channelId int = 0

		c.batchRequestArray = append(c.batchRequestArray, jsonrpcRequest)
		channelId = jsonrpcRequest.Id.(int)
		c.batchResponseMap[channelId] = make(chan Response)

		if !c.batchIsRun {
			c.batchIsRun = true

			go func() {
				time.Sleep(time.Millisecond * time.Duration(timeout))

				c.batchMutex <- true

				jsonrpcRawResponse := c.RawRequest(c.batchRequestArray)
				jsonrpcArrayResponse, ok = jsonrpcRawResponse.([]Response)
				if !ok {
					jsonrpcArrayResponse = []Response{}
					jsonrpcError := Error{
						Code:    -32603,
						Message: "Internal error",
					}

					for _, requestObject := range c.batchRequestArray {
						jsonrpcArrayResponse = append(jsonrpcArrayResponse, Response{
							Jsonrpc: requestObject.Jsonrpc,
							Id:      requestObject.Id,
							Error:   &jsonrpcError,
						})
					}
				}

				for _, value := range jsonrpcArrayResponse {
					if value.Id != nil {
						var channelId int = int(value.Id.(float64))

						if c.batchResponseMap[channelId] != nil {
							c.batchResponseMap[channelId] <- value
						}
					}
				}

				c.batchIsRun = false
				c.batchRequestArray = []Request{}
				<-c.batchMutex
			}()
		}

		<-c.batchMutex

		jsonrpcResponse = <-c.batchResponseMap[channelId]
		delete(c.batchResponseMap, channelId)
	} else {
		jsonrpcRawResponse = c.RawRequest(jsonrpcRequest)
		jsonrpcResponse, ok = jsonrpcRawResponse.(Response)
		if !ok {
			err = errors.New("Unknown response type")
			return
		}
	}

	if jsonrpcResponse.Error != nil {
		rpcError = jsonrpcResponse.Error
		return
	}

	if isCall {
		err = json.Unmarshal(jsonrpcResponse.Result, response)
		if err != nil {
			return
		}
	}

	return
}

func NewClient(transport ClientTransport) *Client {
	return &Client{
		transport: transport,
		callId:    1,

		batchMutex:        make(chan interface{}, 1),
		batchIsRun:        false,
		batchRequestArray: []Request{},
		batchResponseMap:  map[int]chan Response{},
	}
}

func NewClientHttp(endpoint string) *Client {
	return NewClient(NewClientTransportHttp(endpoint))
}
