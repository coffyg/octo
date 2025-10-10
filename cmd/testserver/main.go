package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/coffyg/octo"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Post struct {
	ID      int    `json:"id"`
	UserID  int    `json:"user_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Comment struct {
	ID      int    `json:"id"`
	PostID  int    `json:"post_id"`
	UserID  int    `json:"user_id"`
	Content string `json:"content"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    string `json:"code"`
}

func main() {
	router := octo.NewRouter[any]()

	// Static routes
	router.GET("/", func(ctx *octo.Ctx[any]) {
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    map[string]string{"message": "Welcome to Octo API"},
		})
	})

	router.GET("/health", func(ctx *octo.Ctx[any]) {
		ctx.JSON(200, map[string]interface{}{
			"status": "healthy",
			"uptime": "1h23m45s",
		})
	})

	// List all users
	router.GET("/api/v1/users", func(ctx *octo.Ctx[any]) {
		users := []User{
			{ID: 1, Name: "John Doe", Email: "john@example.com"},
			{ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
			{ID: 3, Name: "Bob Wilson", Email: "bob@example.com"},
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    users,
		})
	})

	// Get single user
	router.GET("/api/v1/users/:id", func(ctx *octo.Ctx[any]) {
		idStr := ctx.Params["id"]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			ctx.JSON(400, ErrorResponse{
				Success: false,
				Error:   "Invalid user ID",
				Code:    "INVALID_ID",
			})
			return
		}

		user := User{
			ID:    id,
			Name:  fmt.Sprintf("User %d", id),
			Email: fmt.Sprintf("user%d@example.com", id),
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    user,
		})
	})

	// Get user posts
	router.GET("/api/v1/users/:id/posts", func(ctx *octo.Ctx[any]) {
		userID, _ := strconv.Atoi(ctx.Params["id"])
		posts := []Post{
			{ID: 1, UserID: userID, Title: "First Post", Content: "Lorem ipsum dolor sit amet"},
			{ID: 2, UserID: userID, Title: "Second Post", Content: "Consectetur adipiscing elit"},
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    posts,
		})
	})

	// Get single post
	router.GET("/api/v1/posts/:id", func(ctx *octo.Ctx[any]) {
		id, _ := strconv.Atoi(ctx.Params["id"])
		post := Post{
			ID:      id,
			UserID:  1,
			Title:   fmt.Sprintf("Post %d", id),
			Content: "This is a sample post content with some Lorem ipsum text to make it more realistic.",
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    post,
		})
	})

	// Get post comments
	router.GET("/api/v1/posts/:id/comments", func(ctx *octo.Ctx[any]) {
		postID, _ := strconv.Atoi(ctx.Params["id"])
		comments := []Comment{
			{ID: 1, PostID: postID, UserID: 2, Content: "Great post!"},
			{ID: 2, PostID: postID, UserID: 3, Content: "Thanks for sharing"},
			{ID: 3, PostID: postID, UserID: 4, Content: "Very informative"},
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    comments,
		})
	})

	// Search endpoint with query params
	router.GET("/api/v1/search", func(ctx *octo.Ctx[any]) {
		query := ctx.QueryParam("q")
		limit := ctx.DefaultQueryParam("limit", "10")
		offset := ctx.DefaultQueryParam("offset", "0")
		
		results := map[string]interface{}{
			"query":   query,
			"limit":   limit,
			"offset":  offset,
			"results": []map[string]string{
				{"id": "1", "title": "Result 1", "type": "post"},
				{"id": "2", "title": "Result 2", "type": "user"},
				{"id": "3", "title": "Result 3", "type": "comment"},
			},
			"total": 42,
		}
		ctx.JSON(200, APIResponse{
			Success: true,
			Data:    results,
		})
	})

	// Wildcard route for static files
	router.GET("/static/*filepath", func(ctx *octo.Ctx[any]) {
		filepath := ctx.Params["filepath"]
		ctx.JSON(200, map[string]interface{}{
			"file":     filepath,
			"size":     "1.2MB",
			"mimetype": "application/octet-stream",
		})
	})

	// Complex nested route
	router.GET("/api/v1/organizations/:org_id/projects/:project_id/tasks/:task_id", func(ctx *octo.Ctx[any]) {
		orgID := ctx.Params["org_id"]
		projectID := ctx.Params["project_id"]
		taskID := ctx.Params["task_id"]
		
		ctx.JSON(200, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"organization_id": orgID,
				"project_id":      projectID,
				"task_id":         taskID,
				"task": map[string]interface{}{
					"title":       "Implement feature X",
					"status":      "in_progress",
					"assignee":    "john.doe",
					"priority":    "high",
					"due_date":    "2024-12-31",
					"tags":        []string{"backend", "api", "urgent"},
					"attachments": 3,
				},
			},
		})
	})

	// 404 handler
	router.ANY("/*", func(ctx *octo.Ctx[any]) {
		ctx.JSON(404, ErrorResponse{
			Success: false,
			Error:   "Endpoint not found",
			Code:    "NOT_FOUND",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8056"
	}
	log.Printf("Starting Octo test server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}