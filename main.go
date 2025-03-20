package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"io"
	"net/http"
	"sync"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Response Wrapper
type Response struct {
	Data    interface{}
	Message string
}

// Global Map for Session info (Session ID -> *gitlab.Client)
var clientStore = struct {
	sync.RWMutex
	clients map[string]*gitlab.Client
}{
	clients: make(map[string]*gitlab.Client),
}

func main() {
	e := echo.New()

	// Set middleware for Logger & Recover
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// Set middleware for static resources
	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Root: "static",
	}))
	// Set middleware for session
	e.Use(session.Middleware(sessions.NewCookieStore([]byte("super-secret-key"))))

	// Set Template Renderer
	renderer := &TemplateRenderer{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e.Renderer = renderer

	// GET: Login Page
	e.GET("/", func(c echo.Context) error {
		// check whether session exists
		_client := getSession(c)
		response := Response{}

		if _client != nil {
			response.Message = "Login Success"
			return c.Render(http.StatusOK, "index.html", response)
		}
		return c.Render(http.StatusOK, "login.html", response)
	})

	// POST: Submit Login Form (token, url)
	e.POST("/login", func(c echo.Context) error {
		data := Response{}
		token := c.FormValue("token")
		baseUrl := c.FormValue("baseUrl")

		if baseUrl == "" || token == "" {
			data.Message = "Gitlab API URL and Private Token should exist"
			return c.Render(http.StatusOK, "login.html", data)
		}

		client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))

		if err != nil {
			data.Message = "클라이언트 생성 오류: " + err.Error()
			return c.Render(http.StatusInternalServerError, "login.html", data)
		}

		// 세션 가져오기
		sess, err := session.Get("session", c)
		if err != nil {
			data.Message = "세션 오류: " + err.Error()
			return c.Render(http.StatusInternalServerError, "login.html", data)
		}

		sessionID, ok := sess.Values["session_id"].(string)
		if !ok || sessionID == "" {
			sessionID = generateSessionID()
			sess.Values["session_id"] = sessionID
			sess.Save(c.Request(), c.Response())
		}

		clientStore.Lock()
		clientStore.clients[sessionID] = client
		clientStore.Unlock()

		return c.Redirect(http.StatusFound, "/")

	})

	// POST: 토큰이 권한을 가지고 있는 모든 프로젝트
	e.GET("/search", func(c echo.Context) error {

		_client := getSession(c)

		if _client != nil {
			projects := Search(_client)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"data":    projects,
				"message": "Search Success",
			})
		}

		return c.Redirect(http.StatusTemporaryRedirect, "/login")
	})

	// e.POST("/delete_package", func(c echo.Context) error {

	// 	projectId := c.FormValue("project_id")
	// 	packageId := c.FormValue("package_id")

	// 	log.Printf("delete package 호출 - projectId: %v, packageId: %v", projectId, packageId)

	// 	response := PageData{
	// 		Data:    "",
	// 		Message: DeletePackageFiles(token, baseUrl, projectId, packageId),
	// 	}

	// 	return c.Render(http.StatusOK, "index.html", response)

	// })

	e.Logger.Fatal(e.Start(":8080"))
}

func getSession(c echo.Context) *gitlab.Client {
	sess, err := session.Get("session", c)
	if err != nil {
		return nil
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		return nil
	}

	clientStore.RLock()
	client, exists := clientStore.clients[sessionID]
	clientStore.RUnlock()

	if !exists {
		return nil
	}

	return client
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
