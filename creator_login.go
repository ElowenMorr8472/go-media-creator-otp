package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Server struct {
	sms    *SMSClient
	assets *AssetStore
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code", s.requestCode)
	mux.HandleFunc("POST /login/verify", s.verifyCode)
	mux.HandleFunc("POST /assets", s.ingestAsset)
	mux.HandleFunc("POST /assets/{id}/jobs", s.advanceJob)
	mux.HandleFunc("POST /assets/{id}/delivery", s.deliverAsset)
	return mux
}

type codeRequest struct {
	Phone     string `json:"phone"`
	RequestID string `json:"request_id"`
}

func (s *Server) requestCode(w http.ResponseWriter, r *http.Request) {
	var input codeRequest
	if !decode(w, r, &input) || input.Phone == "" || input.RequestID == "" {
		writeError(w, http.StatusBadRequest, "phone and request_id are required")
		return
	}
	id, err := s.sms.SendOTP(r.Context(), input.Phone, input.RequestID)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"challenge_id": id, "status": "code_sent"})
}

type verifyRequest struct {
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func (s *Server) verifyCode(w http.ResponseWriter, r *http.Request) {
	var input verifyRequest
	if !decode(w, r, &input) || input.Phone == "" || input.Code == "" || input.RequestID == "" {
		writeError(w, http.StatusBadRequest, "phone, code, and request_id are required")
		return
	}
	valid, err := s.sms.VerifyOTP(r.Context(), input.Phone, input.Code, input.RequestID)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "code was not accepted")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

func (s *Server) ingestAsset(w http.ResponseWriter, r *http.Request) {
	var asset Asset
	if !decode(w, r, &asset) {
		return
	}
	if err := s.assets.Ingest(asset); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	asset.State = AssetIngested
	writeJSON(w, http.StatusCreated, asset)
}

type jobRequest struct {
	State AssetState `json:"state"`
}

func (s *Server) advanceJob(w http.ResponseWriter, r *http.Request) {
	var input jobRequest
	if !decode(w, r, &input) {
		return
	}
	asset, err := s.assets.Advance(r.PathValue("id"), input.State)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

type deliveryRequest struct {
	CreatorID string `json:"creator_id"`
	Phone     string `json:"phone"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func (s *Server) deliverAsset(w http.ResponseWriter, r *http.Request) {
	var input deliveryRequest
	if !decode(w, r, &input) || strings.TrimSpace(input.CreatorID) == "" {
		writeError(w, http.StatusBadRequest, "creator_id and verification fields are required")
		return
	}
	valid, err := s.sms.VerifyOTP(r.Context(), input.Phone, input.Code, input.RequestID)
	if err != nil {
		writeError(w, statusForError(err), err.Error())
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "code was not accepted")
		return
	}
	asset, err := s.assets.Deliver(r.PathValue("id"), input.CreatorID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, asset)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
