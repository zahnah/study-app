package render

import (
	"bytes"
	"fmt"
	"github.com/justinas/nosurf"
	"github.com/zahnah/study-app/internal/config"
	"github.com/zahnah/study-app/internal/models"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

var functions = template.FuncMap{}

var app *config.AppConfig

func NewTemplates(a *config.AppConfig) {
	app = a
}

func AddDefaultData(td *models.TemplateData, r *http.Request) *models.TemplateData {
	td.CSRFToken = nosurf.Token(r)
	return td
}

func Template(writer http.ResponseWriter, r http.Request, tmpl string, td *models.TemplateData) {

	tc := app.TemplateCache
	if !app.UseCache {
		tc, _ = CreateTemplateCache()
	}

	parseTemplate, ok := tc[tmpl]
	if !ok {
		log.Fatal("Couldn't get template for the page")
	}

	buf := new(bytes.Buffer)

	_ = parseTemplate.Execute(buf, AddDefaultData(td, &r))

	_, err := buf.WriteTo(writer)

	if err != nil {
		fmt.Println("error parsing template:", err)
		return
	}
}

func CreateTemplateCache() (map[string]*template.Template, error) {
	myCache := map[string]*template.Template{}

	pages, err := filepath.Glob("./templates/*.page.gohtml")
	if err != nil {
		return myCache, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		ts, err := template.New(name).Funcs(functions).ParseFiles(page)
		if err != nil {
			return myCache, err
		}

		matches, err := filepath.Glob("./templates/*.layout.gohtml")
		if err != nil {
			return myCache, err
		}

		if len(matches) > 0 {
			ts, err = ts.ParseGlob("./templates/*.layout.gohtml")
			if err != nil {
				return myCache, err
			}
		}

		myCache[name] = ts
	}

	return myCache, nil
}