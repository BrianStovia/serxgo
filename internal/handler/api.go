package handler

import (
	"encoding/json"
	"net/http"
)

// ServeAPI handles GET /api/search returning JSON responses
func (h *Handler) ServeAPI(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for public API consumers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	req := h.parseSearchRequest(r)
	if req.Query == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Query parameter 'q' is required",
		})
		return
	}

	resp, err := h.aggregator.Search(r.Context(), req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	if req.Page == 1 {
		resp.InstantAnswers = h.instantService.FindInstantAnswers(r.Context(), req.Query, h.getClientIP(r), r.UserAgent())
		for _, ans := range resp.InstantAnswers {
			if ans.Type == "infobox" {
				resp.Infoboxes = append(resp.Infoboxes, ans)
			} else {
				resp.Answers = append(resp.Answers, ans)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
