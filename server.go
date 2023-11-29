package jsonrpc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
)

type Server struct {
	HandlerMap map[string]Handler
}

func NewServer() (server *Server) {
	server = &Server{
		HandlerMap: map[string]Handler{},
	}

	return
}

func (s *Server) HandleFunc(method string, handle HandlerFunc, request interface{}, response interface{}) {
	var Request, Response reflect.Type = nil, nil

	if request != nil {
		Request = reflect.TypeOf(request)
	}

	if response != nil {
		Response = reflect.TypeOf(response)
	}

	s.HandlerMap[method] = Handler{
		Request:  Request,
		Response: Response,
		Function: handle,
	}
}

func (s *Server) Call(request Request) (response *Response) {
	var responseResult interface{}
	var responseError *Error

	defer func() {
		if response != nil {
			response.Jsonrpc = request.Jsonrpc
			response.Id = request.Id

			if responseResult != nil && responseError == nil {
				jsonRawMessage, err := json.Marshal(responseResult)
				if err != nil {
					responseError = &Error{
						Code:    -32603,
						Message: "Internal error",
					}
				} else {
					response.Result = jsonRawMessage
					return
				}
			}

			if responseError == nil {
				responseError = &Error{
					Code:    -32603,
					Message: "Internal error",
				}
			}

			response.Error = responseError
			return
		}
	}()

	if request.Jsonrpc != "2.0" {
		response = &Response{}
		responseError = &Error{
			Code:    -32600,
			Message: "Invalid Request",
			Data:    "Invalid JsonRPC version",
		}
		return
	}

	if request.Id != nil {
		response = &Response{}
	}

	handler, ok := s.HandlerMap[request.Method]
	if !ok {
		responseError = &Error{
			Code:    -32601,
			Message: "Method not found",
			Data:    request.Method,
		}
		return
	}

	var err error
	var requestParams interface{}
	if handler.Request != nil {
		reflectParams := reflect.New(handler.Request).Elem()
		err = json.Unmarshal(request.Params, reflectParams.Addr().Interface())
		if err != nil {
			responseError = &Error{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			}
			return
		}
		requestParams = reflectParams.Interface()
	}

	responseResult, err = handler.Function(requestParams)
	if err != nil {
		var ok bool
		responseError, ok = err.(*Error)
		if !ok {
			responseError = &Error{
				Code:    -32603,
				Message: "Internal error",
				Data:    err.Error(),
			}
		}
		return
	}

	return
}

func (s *Server) Handler(request []byte) (response []byte) {
	var responseInterface interface{}
	var errorObject error
	var unmarshalError, marshalError bool
	_ = marshalError

	defer func() {
		if errorObject != nil {
			responseStruct := &Response{
				Jsonrpc: "2.0",
			}

			if unmarshalError {
				if json.Valid(request) {
					responseStruct.Error = &Error{
						Code:    -32600,
						Message: "Invalid Request",
						Data:    errorObject.Error(),
					}
				} else {
					responseStruct.Error = &Error{
						Code:    -32700,
						Message: "Parse error",
						Data:    errorObject.Error(),
					}
				}
			} else {
				responseStruct.Error = &Error{
					Code:    -32603,
					Message: "Internal error",
					Data:    errorObject.Error(),
				}
			}

			response, errorObject = json.Marshal(responseStruct)
			if errorObject != nil {
				return
			}
		}
	}()

	if len(request) == 0 {
		unmarshalError = true
		errorObject = errors.New("Empty request")
		return
	}

	isBatch := request[0] == '['

	if isBatch {
		requestArrayStruct := []Request{}
		errorObject = json.Unmarshal(request, &requestArrayStruct)
		if errorObject != nil {
			unmarshalError = true
			return
		}

		if len(requestArrayStruct) == 0 {
			errorObject = errors.New("Empty request")
			unmarshalError = true
			return
		}

		var responseArrayStruct []*Response = make([]*Response, 0)
		for _, requestStruct := range requestArrayStruct {
			responseStruct := s.Call(requestStruct)
			if responseStruct != nil {
				responseArrayStruct = append(responseArrayStruct, responseStruct)
			}
		}
		if responseArrayStruct != nil {
			responseInterface = responseArrayStruct
		}
	} else {
		requestStruct := Request{}
		errorObject = json.Unmarshal(request, &requestStruct)
		if errorObject != nil {
			unmarshalError = true
			return
		}

		responseStruct := s.Call(requestStruct)

		if responseStruct != nil {
			responseInterface = responseStruct
		}
	}

	if responseInterface != nil {
		response, errorObject = json.Marshal(responseInterface)
		if errorObject != nil {
			marshalError = true
			return
		}
	}

	return
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		request, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(rw, "Server error", http.StatusInternalServerError)
			return
		}

		response := s.Handler(request)
		rw.Write(response)
	}
}
