package main

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
)

type Page struct {
	Name     string
	Title    string
	Body     string
	HtmlBody template.HTML
}

var (
	templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html"))
	validPath = regexp.MustCompile("^/(edit|save|view)/(\\w{1,20})$")
	db        *sql.DB
)

func init() {
	var err error
	if _, err = os.Stat("./data/wiki.db"); os.IsNotExist(err) {
		initDb()
	}

	db, err = sql.Open("sqlite3", "./data/wiki.db")
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
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
		name text not null primary key,
		title text not null,
		body text not null);
	delete from pages;
	`

	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func goHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name != "" {
		http.Redirect(w, r, "/view/"+name, http.StatusFound)
	} else {
		http.Redirect(w, r, "/view/front", http.StatusFound)
	}

}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	name, err := getName(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(name)
	if err != nil {
		http.Redirect(w, r, "/edit/"+name, http.StatusFound)
		return
	}
	renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request) {
	name, err := getName(w, r)
	if err != nil {
		return
	}
	p, err := loadPage(name)
	if err != nil {
		p = &Page{Name: name}
	}
	renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	name, err := getName(w, r)
	if err != nil {
		return
	}

	if r.FormValue("submit") == "Cancel" {
		http.Redirect(w, r, "/view/"+name, http.StatusFound)
	} else {
		body := r.FormValue("body")
		title := r.FormValue("title")
		p := &Page{Name: name, Title: title, Body: body}
		err = p.save()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/view/"+name, http.StatusFound)
	}
}

func getName(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("Invalid Page Name")
	}
	return m[2], nil
}

func (p *Page) save() error {
	result, err := db.Exec(
		"UPDATE pages SET name = $1, title = $2, body= $3 WHERE name = $1",
		p.Name, p.Title, p.Body)
	log.Println("name: " + p.Name + " title: " + p.Title)
	if err != nil {
		log.Fatal(err)
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	if rowCount == 0 {
		log.Println("INSERT new page")
		_, err := db.Exec(
			"INSERT OR IGNORE INTO pages VALUES($1, $2, $3)",
			p.Name, p.Title, p.Body)
		if err != nil {
			log.Fatal(err)
		}
	}

	return err
}

func loadPage(name string) (*Page, error) {
	page := &Page{Name: name, Title: "", Body: ""}
	row := db.QueryRow("SELECT * FROM pages WHERE name = ?", name)

	err := row.Scan(&page.Name, &page.Title, &page.Body)
	if err == sql.ErrNoRows {
		log.Println("No Rows found for page")
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}
	temp := []byte(page.Body)
	temp = blackfriday.MarkdownCommon(temp)
	temp = bluemonday.UGCPolicy().SanitizeBytes(temp)

	page.HtmlBody = template.HTML(temp)
	return page, nil
}
