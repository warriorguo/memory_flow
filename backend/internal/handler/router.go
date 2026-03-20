package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/warriorguo/memory_flow/backend/internal/middleware"
)

func NewRouter(
	projectHandler *ProjectHandler,
	issueHandler *IssueHandler,
	progressHandler *ProgressHandler,
	memoryHandler *MemoryHandler,
	tagHandler *TagHandler,
) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.CORS)

	r.Route("/api/v1", func(r chi.Router) {
		// Projects
		r.Get("/projects", projectHandler.List)
		r.Post("/projects", projectHandler.Create)
		r.Get("/projects/{id}", projectHandler.Get)
		r.Put("/projects/{id}", projectHandler.Update)
		r.Delete("/projects/{id}", projectHandler.Archive)

		// Issues (scoped by project for creation/listing)
		r.Get("/projects/{projectId}/issues", issueHandler.ListByProject)
		r.Post("/projects/{projectId}/issues", issueHandler.Create)

		// Issues (direct access)
		r.Get("/issues", issueHandler.Search)
		r.Get("/issues/{id}", issueHandler.Get)
		r.Put("/issues/{id}", issueHandler.Update)
		r.Patch("/issues/{id}/status", issueHandler.TransitionStatus)
		r.Get("/issues/{id}/history", issueHandler.GetHistory)

		// Issue tags
		r.Post("/issues/{id}/tags", tagHandler.AddToIssue)
		r.Delete("/issues/{id}/tags/{tagId}", tagHandler.RemoveFromIssue)

		// Progress
		r.Get("/projects/{projectId}/progress/summary", progressHandler.GetSummary)
		r.Get("/projects/{projectId}/progress/trend", progressHandler.GetTrend)

		// Tags
		r.Get("/tags", tagHandler.List)
		r.Post("/tags", tagHandler.Create)

		// Memories
		r.Get("/memories", memoryHandler.List)
		r.Post("/memories", memoryHandler.Create)
		r.Get("/memories/{id}", memoryHandler.Get)
		r.Put("/memories/{id}", memoryHandler.Update)
		r.Delete("/memories/{id}", memoryHandler.Delete)

		// Memory tags
		r.Post("/memories/{id}/tags", tagHandler.AddToMemory)
		r.Delete("/memories/{id}/tags/{tagId}", tagHandler.RemoveFromMemory)
	})

	return r
}
