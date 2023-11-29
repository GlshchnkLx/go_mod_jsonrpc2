# Golang JsonRPC2 Client/Server

This module provides a simple implementation of the [JsonRPC2](https://www.jsonrpc.org) specification. The main goal is to provide a minimum of third-party dependencies and a simple logic of integration into the code

## Installation

```
go get github.com/GlshchnkLx/go_mod_jsonrpc2
```

```golang
import (
	jsonrpc "github.com/GlshchnkLx/go_mod_jsonrpc2"
)
```

## Server Examples

```golang
package main

import (
	"fmt"
	"net/http"

	jsonrpc "github.com/GlshchnkLx/go_mod_jsonrpc2"
)

func main() {
	// Creating a server object
	rpcServer := jsonrpc.NewServer()

	// We register a simple handler that does not accept parameters and returns a string
	rpcServer.HandleFunc("hello", func(i interface{}) (interface{}, error) {
		return "world", nil
	}, nil, "")

	// A handler that accepts a int and returns a int
	rpcServer.HandleFunc("mul2", func(i interface{}) (interface{}, error) {
		value := i.(int)
		return value * 15, nil
	}, 0, 0)

	// A handler that accepts an array of int and returns a int
	rpcServer.HandleFunc("sum", func(i interface{}) (interface{}, error) {
		valueArray := i.([]int)
		output := 0

		for _, value := range valueArray {
			output += value
		}

		return output, nil
	}, []int{}, 0)

	//The handler accepts the structure and returns the string + some error
	{
		type Request struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
			City string `json:"city"`
		}

		rpcServer.HandleFunc("hiHuman", func(i interface{}) (interface{}, error) {
			human := i.(Request)

			if human.Name == "" {
				return nil, &jsonrpc.Error{
					Code:    100,
					Message: "name must been",
				}
			}

			if human.City == "" {
				return nil, &jsonrpc.Error{
					Code:    101,
					Message: "city must been",
				}
			}

			return fmt.Sprintf("Hello %s from %s. You are %d years old",
				human.Name,
				human.City,
				human.Age,
			), nil
		}, Request{}, "")
	}

	http.Handle("/api", rpcServer)

	http.ListenAndServe(":8080", nil)
}
```