package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"nvr/internal/model"
)

func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, username, role, enabled, created_at, updated_at FROM users ORDER BY id`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}
	writeJSON(w, http.StatusOK, users)
}

// bootstrap allows creating the first admin user when no users exist.
func (h *handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count > 0 {
		http.Error(w, "bootstrap disabled — users already exist", http.StatusForbidden)
		return
	}
	h.addUser(w, r)
}

func (h *handler) addUser(w http.ResponseWriter, r *http.Request) {
	var u model.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if u.Username == "" || u.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if u.Role == "" {
		u.Role = model.RoleViewer
	}
	if u.Role != model.RoleAdmin && u.Role != model.RoleViewer {
		http.Error(w, "role must be admin or viewer", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	result, err := h.db.Exec(
		`INSERT INTO users (username, password, role, enabled) VALUES (?, ?, ?, ?)`,
		u.Username, string(hash), u.Role, true,
	)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "username already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u.ID, _ = result.LastInsertId()
	u.Password = ""
	u.Enabled = true
	writeJSON(w, http.StatusCreated, u)
}

func (h *handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var u model.User
	err = h.db.QueryRow(
		`SELECT id, username, role, enabled, created_at, updated_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *handler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var u model.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if u.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		_, err = h.db.Exec(
			`UPDATE users SET username=?, password=?, role=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			u.Username, string(hash), u.Role, u.Enabled, id,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		_, err = h.db.Exec(
			`UPDATE users SET username=?, role=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			u.Username, u.Role, u.Enabled, id,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	_, err = h.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func isUniqueViolation(err error) bool {
	return err != nil && (err.Error() == "UNIQUE constraint failed: users.username" ||
		// ncruces/go-sqlite3 wraps the error
		len(err.Error()) > 0 && err.Error()[0:6] == "UNIQUE")
}
