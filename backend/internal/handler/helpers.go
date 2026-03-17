package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

type listResponse struct {
	Data     interface{} `json:"data"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

type dataResponse struct {
	Data interface{} `json:"data"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeData(w http.ResponseWriter, status int, data interface{}) {
	writeJSON(w, status, dataResponse{Data: data})
}

func writeList(w http.ResponseWriter, data interface{}, total, page, pageSize int) {
	writeJSON(w, http.StatusOK, listResponse{
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func decodeJSON(r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB limit
	return json.NewDecoder(r.Body).Decode(v)
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func queryString(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

func queryUUID(r *http.Request, key string) *uuid.UUID {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil
	}
	return &id
}
