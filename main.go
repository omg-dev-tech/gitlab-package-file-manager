package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"flag"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

// Response Wrapper
type Response struct {
	Data      interface{}
	Message   string
	CsrfToken string
}

type Request[T any] struct {
	Data T `json:"data"`
}

// Global Map for Session info (Session ID -> *gitlab.Client)
var clientStore = struct {
	sync.RWMutex
	clients map[string]*gitlab.Client
	tokens  map[string]string
}{
	clients: make(map[string]*gitlab.Client),
	tokens:  make(map[string]string),
}

func main() {
	// TODO DB Enable flag 처리

	// dsn := "host=localhost user=postgres dbname=postgres port=5432 sslmode=disable TimeZone=Asia/Seoul"
	// db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	port := flag.Int("port", 8080, "Web Port")
	flag.Parse()

	e := echo.New()

	// Set middleware for Logger & Recover
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	// Set middleware for session
	store := sessions.NewCookieStore([]byte("super-secret-key"))
	store.Options = &sessions.Options{
		Secure:   false,
		HttpOnly: true,
	}
	e.Use(session.Middleware(store))

	// CSRF 미들웨어 제거 또는 비활성화
	e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup: "header:X-XSRF-TOKEN,form:X-XSRF-TOKEN",
		CookieName:  "XSRF-TOKEN", // 쿠키에 저장되는 토큰 이름
	}))

	// fs.FS에서 http.FileSystem으로 변환 (서브 디렉토리 사용 가능)
	staticFiles, _ := fs.Sub(staticFS, "static")
	e.StaticFS("/static", staticFiles)

	// Set Template Renderer
	renderer := &TemplateRenderer{
		templates: template.Must(template.ParseFS(templatesFS, "templates/*.html")),
	}
	e.Renderer = renderer

	// GET: Login Page
	e.GET("/", func(c echo.Context) error {
		// check whether session exists
		_client := getClient(c)
		csrfToken := c.Get("csrf").(string)
		response := Response{
			CsrfToken: csrfToken,
		}
		if _client != nil {
			response.Message = "Login Success"
			return c.Render(http.StatusOK, "index.html", response)
		}
		return c.Render(http.StatusOK, "login.html", response)
	})

	// POST: Submit Login Form (token, url)
	e.POST("/login", func(c echo.Context) error {
		csrfToken := c.FormValue("X-XSRF-TOKEN")
		response := Response{
			CsrfToken: csrfToken,
		}
		token := c.FormValue("token")
		baseUrl := c.FormValue("baseUrl")

		log.Printf("baseUrl: %v, token: %v", baseUrl, token)

		if baseUrl == "" || token == "" {
			log.Printf("입력 오류!!")
			response.Message = "Gitlab API URL and Private Token should exist"
			return c.Render(http.StatusOK, "login.html", response)
		}

		client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))

		if err != nil {
			log.Printf("클라이언트 생성 오류: %v", err.Error())
			response.Message = "클라이언트 생성 오류: " + err.Error()
			return c.Render(http.StatusInternalServerError, "login.html", response)
		}

		if _, _, err := client.Users.CurrentUser(); err != nil {
			response.Message = "올바르지 않은 토큰 값입니다."
			return c.Render(http.StatusUnauthorized, "login.html", response)
		}

		// 세션 가져오기
		sess, err := session.Get("session", c)
		if err != nil {
			response.Message = "세션 오류: " + err.Error()
			return c.Render(http.StatusInternalServerError, "login.html", response)
		}

		sessionID, ok := sess.Values["session_id"].(string)
		if !ok || sessionID == "" {
			sessionID = generateSessionID()
			sess.Values["session_id"] = sessionID
			sess.Save(c.Request(), c.Response())
		}

		clientStore.Lock()
		clientStore.clients[sessionID] = client
		clientStore.tokens[sessionID] = token
		clientStore.Unlock()

		return c.Redirect(http.StatusFound, "/")

	})

	e.POST("/logout", func(c echo.Context) error {
		// CSRF 토큰은 필요한 경우 로그 출력이나 추가 검증에 사용 가능
		csrfToken := c.FormValue("X-XSRF-TOKEN")
		log.Printf("Logout 요청 CSRF 토큰: %v", csrfToken)

		// 세션 가져오기
		sess, err := session.Get("session", c)
		if err != nil {
			// 세션이 이미 파기된 경우도 있으므로 에러 로깅 후 리다이렉트
			log.Printf("세션 가져오기 오류: %v", err)
			return c.Redirect(http.StatusFound, "/")
		}

		// 세션에 저장된 session_id를 통해 clientStore에서 삭제
		sessionID, ok := sess.Values["session_id"].(string)
		if ok && sessionID != "" {
			clientStore.Lock()
			delete(clientStore.clients, sessionID)
			clientStore.Unlock()
		}

		// 세션을 삭제 (쿠키 삭제를 위해 MaxAge를 -1로 설정)
		sess.Options.MaxAge = -1
		if err := sess.Save(c.Request(), c.Response()); err != nil {
			log.Printf("세션 삭제 오류: %v", err)
		}

		return c.Redirect(http.StatusFound, "/")
	})

	// POST: 토큰이 권한을 가지고 있는 모든 프로젝트
	e.GET("/search", func(c echo.Context) error {

		_client := getClient(c)

		csrfToken := c.Get("csrf").(string)
		if _client != nil {

			projectName := c.FormValue("projectName")
			packageName := c.FormValue("packageName")
			fromFileCount := c.FormValue("fromFileCount")
			toFileCount := c.FormValue("toFileCount")

			projects := Search(_client, projectName, packageName, fromFileCount, toFileCount)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"data":      projects,
				"message":   "Search Success",
				"CsrfToken": csrfToken,
			})
		}

		return c.Redirect(http.StatusTemporaryRedirect, "/login")
	})

	e.POST("/clean", func(c echo.Context) error {
		var request Request[[]PackageFile]
		_client := getClient(c)
		csrfToken := c.Get("csrf").(string)
		if err := c.Bind(&request); err != nil {
			log.Printf("error: %v", err)
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"data":      err,
				"message":   "Clean Fail",
				"CsrfToken": csrfToken,
			})
		}

		log.Printf("Input Data: %v", request)
		if _client != nil {
			results := Clean(_client, request.Data)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"data":      results,
				"message":   "Clean Success",
				"CsrfToken": csrfToken,
			})
		}

		return c.Redirect(http.StatusTemporaryRedirect, "/login")

	})

	e.GET("/statistics", func(c echo.Context) error {
		_client := getClient(c)
		Statistics(_client)

		return c.Render(http.StatusOK, "/main.html", "")
	})

	e.Logger.Fatal(e.Start(":" + strconv.Itoa(*port)))
}

func getClient(c echo.Context) *gitlab.Client {
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

func getToken(c echo.Context) *string {
	sess, err := session.Get("session", c)
	if err != nil {
		return nil
	}

	sessionID, ok := sess.Values["session_id"].(string)
	if !ok || sessionID == "" {
		return nil
	}

	clientStore.RLock()
	token, exists := clientStore.tokens[sessionID]
	clientStore.RUnlock()

	if !exists {
		return nil
	}

	return &token
}

func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
