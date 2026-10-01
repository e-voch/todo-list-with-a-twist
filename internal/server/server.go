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

// NewHandler returns the application's HTTP routes. Reads come from store;
// writes are sent as events through pub.
func NewHandler(store task.Store, pub Publisher) http.Handler {
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err) // the embedded directory is fixed at build time
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/delete/", deleteHandler(pub))
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
			// The ID is chosen here, before the event is published, so a
			// redelivered event creates the same task instead of a duplicate.
			id, err := uuid.NewV7()
			if err != nil {
				slog.Error("generate id failed", "err", err)
				http.Error(w, "could not create task", http.StatusInternalServerError)
				return
			}
			event := task.EventMessage{
				Type: task.EventCreated,
				Task: task.Task{ID: id, Title: title, Description: description},
			}
			if err := pub.Publish(event); err != nil {
				slog.Error("publish create failed", "err", err)
				http.Error(w, "could not create task", http.StatusInternalServerError)
				return
			}
			slog.Info("create event published", "id", id, "title", title, "description", description)
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

func deleteHandler(pub Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("delete endpoint reached", "method", r.Method, "path", r.URL.Path)
		idStr := r.FormValue("id")

		id, err := uuid.Parse(idStr)
		if err != nil {
			slog.Error("invalid id", "id", idStr, "err", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		event := task.EventMessage{Type: task.EventDeleted, Task: task.Task{ID: id}}
		if err := pub.Publish(event); err != nil {
			slog.Error("publish delete failed", "id", id, "err", err)
			http.Error(w, "could not delete task", http.StatusInternalServerError)
			return
		}
		slog.Info("delete event published", "id", id)
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
			event := task.EventMessage{
				Type: task.EventEdited,
				Task: task.Task{ID: id, Title: title, Description: description},
			}
			if err := pub.Publish(event); err != nil {
				slog.Error("publish edit failed", "id", id, "err", err)
				http.Error(w, "could not update task", http.StatusInternalServerError)
				return
			}
			slog.Info("edit event published", "id", id, "title", title, "description", description)
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
