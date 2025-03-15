package main

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

type Project struct {
	ProjectId          int
	ProjectName        string
	ProjectAccessLevel int
	Packages           []Package
}

type Package struct {
	PackageId    int
	PackageName  string
	PackageFiles []PackageFile
}

type PackageFile struct {
	PackageFileId   int
	PackageFileName string
}

type PageData struct {
	Token    string
	Projects []Project
	Message  string
}

func main() {
	e := echo.New()

	// 미들웨어 설정 (로깅, Recover 등)
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 템플릿 렌더러 설정
	renderer := &TemplateRenderer{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e.Renderer = renderer

	// GET: 토큰 입력 및 조회 폼 표시
	e.GET("/", func(c echo.Context) error {
		data := PageData{}
		return c.Render(http.StatusOK, "index.html", data)
	})

	// POST: 토큰이 권한을 가지고 있는 모든 프로젝트
	e.POST("/search", func(c echo.Context) error {
		token := c.FormValue("token")
		// baseUrl := c.FormValue("base-url")
		baseUrl := "https://git.bwg.co.kr/gitlab/api/v4"

		projects := Search(token, baseUrl)
		data := PageData{
			Token:    token,
			Projects: projects,
			Message:  "조회가 완료되었습니다.",
		}

		return c.Render(http.StatusOK, "index.html", data)

	})

	e.Logger.Fatal(e.Start(":8080"))
}
