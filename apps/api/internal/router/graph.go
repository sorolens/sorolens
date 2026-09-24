package router

import (
	"net/http"
)

func ContractGraphHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"nodes": [{"id": "contractA"}], "edges": []}`))
}
