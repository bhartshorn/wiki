package main

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
)

type Page struct {
	Title    string
	Body     []byte
	HtmlBody template.HTML
}

var (
	templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/(\\w{1,20})$")
	db        *sql.DB
)

func init() {
	if _, err := os.Stat("./data/wiki.db"); os.IsNotExist(err) {
		initDb()
	}

	db, err := sql.Open("sqlite3", "./data/wiki.db")
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
}

func initDb() {
	log.Println("Creating DB for the first time")
	db, err := sql.Open("sqlite3", "./data/wiki.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	createTable := `
	create table pages (
		title text not null primary key,
		body text not null);
	delete from pages;
	`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/edit/", editHandler)
	http.HandleFunc("/save/", saveHandler)
	http.HandleFunc("/go/", goHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.ListenAndServe(":8080", nil)
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func goHandler(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	if title != "" {
		http.Redirect(w, r, "/view/"+title, http.StatusFound)
	} else {
		http.Redirect(w, r, "/view/front", http.StatusFound)
	}

}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	title, err := getTitle(w, r)
	if err != nil {
		return
	}

	if r.FormValue("submit") == "Cancel" {
		http.Redirect(w, r, "/view/"+title, http.StatusFound)
	} else {
		body := r.FormValue("body")
		p := &Page{Title: title, Body: []byte(body)}
		err = p.save()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/view/"+title, http.StatusFound)
	}
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("Invalid Page Title")
	}
	return m[2], nil
}

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	unsafe := blackfriday.MarkdownCommon(body)
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	return &Page{Title: title, Body: body, HtmlBody: template.HTML(html)}, nil
}
