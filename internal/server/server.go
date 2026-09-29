package server

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"todo/internal/task"
	"todo/web"
)

// NewHandler returns the application's HTTP routes backed by store.
func NewHandler(store task.Store) http.Handler {
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err) // the embedded directory is fixed at build time
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/delete/", deleteHandler(store))
	mux.HandleFunc("/edit/", editHandler(store))
	mux.HandleFunc("/{$}", homeHandler(store))
	mux.HandleFunc("/", notFoundHandler)
	return mux
}

func homeHandler(store task.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("home endpoint reached", "method", r.Method, "path", r.URL.Path)
		title := r.FormValue("ftitle")
		description := r.FormValue("fdescription")

		if title != "" {
			id, err := store.Create(title, description)
			if err != nil {
				slog.Error("create failed", "err", err)
				http.Error(w, "could not create task", http.StatusInternalServerError)
				return
			}
			slog.Info("task created", "id", id, "title", title, "description", description)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		taskList, err := store.List()
		if err != nil {
			slog.Error("list failed", "err", err)
			http.Error(w, "could not load tasks", http.StatusInternalServerError)
			return
		}

		tmpl, err := template.ParseFS(web.FS, "templates/home.html")
		if err != nil {
			slog.Error("template parse failed", "err", err)
			http.Error(w, "could not render page", http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, taskList); err != nil {
			slog.Error("render failed", "err", err)
		}
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("404 not found", "method", r.Method, "path", r.URL.Path)
	http.Error(w, "404 - Page Not Found", http.StatusNotFound)
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("favicon endpoint reached", "method", r.Method, "path", r.URL.Path)
	w.WriteHeader(http.StatusNoContent)
}

func deleteHandler(store task.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("delete endpoint reached", "method", r.Method, "path", r.URL.Path)
		idStr := r.FormValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			slog.Error("invalid id", "id", idStr, "err", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if err := store.Delete(id); err != nil {
			slog.Error("delete failed", "id", id, "err", err)
			http.Error(w, "could not delete task", http.StatusInternalServerError)
			return
		}
		slog.Info("task deleted", "id", id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func editHandler(store task.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("edit endpoint reached", "method", r.Method, "path", r.URL.Path)
		idStr := r.FormValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			slog.Error("invalid id", "id", idStr, "err", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		title := r.FormValue("ftitle")
		description := r.FormValue("fdescription")

		if title != "" {
			if err := store.Update(id, title, description); err != nil {
				slog.Error("update failed", "id", id, "err", err)
				http.Error(w, "could not update task", http.StatusInternalServerError)
				return
			}
			slog.Info("task updated", "id", id, "title", title, "description", description)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		t, err := store.Get(id)
		if err != nil {
			// The stores don't yet share a "not found" error, so any Get
			// failure is reported as a missing task.
			slog.Error("get failed", "id", id, "err", err)
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}

		tmpl, err := template.ParseFS(web.FS, "templates/edit.html")
		if err != nil {
			slog.Error("template parse failed", "err", err)
			http.Error(w, "could not render page", http.StatusInternalServerError)
			return
		}
		data := struct {
			ID   int
			Task task.Task
		}{id, t}

		slog.Info("edit form displayed", "id", id)
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error("render failed", "err", err)
		}
	}
}
