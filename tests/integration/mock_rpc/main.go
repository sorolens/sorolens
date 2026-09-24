package main

import (
	"encoding/json"
	"net/http"
	"io"
	"log"
)

type JSONRPCReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type JSONRPCResp struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCReq
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		resp := JSONRPCResp{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		if req.Method == "getLatestLedger" {
			resp.Result = map[string]interface{}{
				"id": "123456",
				"protocolVersion": 20,
				"sequence": 1000000,
			}
		} else if req.Method == "getEvents" {
			resp.Result = map[string]interface{}{
				"events": []interface{}{},
			}
		} else {
			resp.Result = map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	log.Println("Mock RPC listening on :8000")
	http.ListenAndServe(":8000", nil)
}
