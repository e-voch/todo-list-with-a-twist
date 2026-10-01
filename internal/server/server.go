package server

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"todo/internal/task"
	"todo/web"
)

// Publisher sends task events to the queue. *queue.Producer satisfies it.
type Publisher interface {
	Publish(e task.EventMessage) error
}

// NewHandler returns the application's HTTP routes. Reads and writes go to
// store; after a successful write, an event is published through pub.
func NewHandler(store task.Store, pub Publisher) http.Handler {
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err) // the embedded directory is fixed at build time
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/delete/", deleteHandler(store, pub))
	mux.HandleFunc("/edit/", editHandler(store, pub))
	mux.HandleFunc("/{$}", homeHandler(store, pub))
	mux.HandleFunc("/", notFoundHandler)
	return mux
}

func homeHandler(store task.Store, pub Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("home endpoint reached", "method", r.Method, "path", r.URL.Path)
		title := r.FormValue("ftitle")
		description := r.FormValue("fdescription")

		if title != "" {
			id, err := uuid.NewV7()
			if err != nil {
				slog.Error("generate id failed", "err", err)
				http.Error(w, "could not create task", http.StatusInternalServerError)
				return
			}
			if err := store.Create(id, title, description); err != nil {
				slog.Error("create failed", "err", err)
				http.Error(w, "could not create task", http.StatusInternalServerError)
				return
			}
			slog.Info("task created", "id", id, "title", title, "description", description)
			publish(pub, task.EventMessage{
				Type: task.EventCreated,
				Task: task.Task{ID: id, Title: title, Description: description},
			})
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

func deleteHandler(store task.Store, pub Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("delete endpoint reached", "method", r.Method, "path", r.URL.Path)
		idStr := r.FormValue("id")

		id, err := uuid.Parse(idStr)
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
		publish(pub, task.EventMessage{Type: task.EventDeleted, Task: task.Task{ID: id}})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func editHandler(store task.Store, pub Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("edit endpoint reached", "method", r.Method, "path", r.URL.Path)
		idStr := r.FormValue("id")

		id, err := uuid.Parse(idStr)
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
			publish(pub, task.EventMessage{
				Type: task.EventEdited,
				Task: task.Task{ID: id, Title: title, Description: description},
			})
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
			ID   uuid.UUID
			Task task.Task
		}{id, t}

		slog.Info("edit form displayed", "id", id)
		if err := tmpl.Execute(w, data); err != nil {
			slog.Error("render failed", "err", err)
		}
	}
}

// publish sends e after the store write has already succeeded. A failure is
// only logged: the task is saved, so the request still succeeds, but the
// event is lost. An outbox table would close that gap.
func publish(pub Publisher, e task.EventMessage) {
	if err := pub.Publish(e); err != nil {
		slog.Error("publish failed", "type", e.Type, "id", e.Task.ID, "err", err)
	}
}
