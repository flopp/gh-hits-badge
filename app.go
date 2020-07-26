package main

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"strings"
)

type App struct {
	router *mux.Router
	db     *sql.DB
}

func (a *App) Initialize(dbFile string) {
	a.router = mux.NewRouter()
	a.initializeRoutes()

	var err error
	a.db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		panic(err)
	}
	a.initializeDB()
}

func (a *App) Run(port int) {
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), a.router))
}

func (a *App) serveSVG(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	user := pathParams["user"]
	repo := pathParams["repo"]

	count, err := a.incrCount(user, repo)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"user": "%s", "repo": "%s", "error": %s}`, user, repo, err)))
	} else {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("cache-control", "max-age=0, no-cache, no-store, must-revalidate")
		svg := a.createSVG(count)
		w.Write([]byte(svg))
	}
}

func (a *App) serveUserRepoStats(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	user := pathParams["user"]
	repo := pathParams["repo"]

	w.Header().Set("Content-Type", "application/json")

	count, err := a.getCount(user, repo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
	} else {
		w.Write([]byte(fmt.Sprintf("{\"%s\": {\n  \"%s\": %d\n}\n}", user, repo, count)))
	}
}

func (a *App) serveUserStats(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	user := pathParams["user"]

	w.Header().Set("Content-Type", "application/json")

	counts, err := a.getRepoCounts(user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
	} else {
		w.Write([]byte(fmt.Sprintf("{\n  \"%s\": {", user)))
		for i, item := range counts {
			if i == 0 {
				w.Write([]byte(fmt.Sprintf("\n    \"%s\": %d", item.repo, item.count)))
			} else {
				w.Write([]byte(fmt.Sprintf(",\n    \"%s\": %d", item.repo, item.count)))
			}
		}
		w.Write([]byte("\n  }\n}"))
	}
}

func (a *App) serveStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	counts, err := a.getUserRepoCounts()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"error\": \"%s\"}", err)))
	} else {
		w.Write([]byte("{"))
		last_user := ""
		for i, item := range counts {
			if i == 0 {
				last_user = item.user
				w.Write([]byte(fmt.Sprintf("\n  \"%s\": {", item.user)))
				w.Write([]byte(fmt.Sprintf("\n    \"%s\": %d", item.repo, item.count)))
			} else if item.user == last_user {
				w.Write([]byte(fmt.Sprintf(",\n    \"%s\": %d", item.repo, item.count)))
			} else {
				last_user = item.user
				w.Write([]byte(fmt.Sprintf(",\n  \"%s\": {", item.user)))
				w.Write([]byte(fmt.Sprintf("\n    \"%s\": %d", item.repo, item.count)))
			}
		}
		if last_user != "" {
			w.Write([]byte("\n  }\n}"))	
		} else {
			w.Write([]byte("\n}"))
		}
	}
}

func (a *App) initializeRoutes() {
	a.router.HandleFunc("/badge/{user:[A-ZA-z0-9_.-]+}/{repo:[A-ZA-z0-9_.-]+}.svg", a.serveSVG).Methods(http.MethodGet)
	a.router.HandleFunc("/stats/{user:[A-ZA-z0-9_.-]+}/{repo:[A-ZA-z0-9_.-]+}/", a.serveUserRepoStats).Methods(http.MethodGet)
	a.router.HandleFunc("/stats/{user:[A-ZA-z0-9_.-]+}/{repo:[A-ZA-z0-9_.-]+}", a.serveUserRepoStats).Methods(http.MethodGet)
	a.router.HandleFunc("/stats/{user:[A-ZA-z0-9_.-]+}/", a.serveUserStats).Methods(http.MethodGet)
	a.router.HandleFunc("/stats/{user:[A-ZA-z0-9_.-]+}", a.serveUserStats).Methods(http.MethodGet)
	a.router.HandleFunc("/stats/", a.serveStats).Methods(http.MethodGet)
	a.router.HandleFunc("/stats", a.serveStats).Methods(http.MethodGet)
}

func (a *App) initializeDB() {
	statement, err := a.db.Prepare("CREATE TABLE IF NOT EXISTS counts (user TEXT NOT NULL, repo TEXT NOT NULL, count INTEGER, PRIMARY KEY(user, repo))")
	if err != nil {
		panic(err)
	}
	_, err = statement.Exec()
	if err != nil {
		panic(err)
	}
}

func (a *App) getCount(user, repo string) (int, error) {
	count := 0
	err := a.db.QueryRow("SELECT count FROM counts WHERE user = ? AND repo = ?", user, repo).Scan(&count)

	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return -1, err
	}

	return count, nil
}

type RepoCount struct {
	repo  string
	count int
}

type UserRepoCount struct {
	user  string
	repo  string
	count int
}

func (a *App) getRepoCounts(user string) ([]*RepoCount, error) {
	rows, err := a.db.Query("SELECT repo, count FROM counts WHERE user = ? ORDER BY repo", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*RepoCount{}
	for rows.Next() {
		var (
			repo  string
			count int
		)
		if err := rows.Scan(&repo, &count); err != nil {
			log.Fatal(err)
		} else {
			item := new(RepoCount)
			item.repo = repo
			item.count = count
			result = append(result, item)
		}
	}
	return result, nil
}

func (a *App) getUserRepoCounts() ([]*UserRepoCount, error) {
	rows, err := a.db.Query("SELECT user, repo, count FROM counts ORDER BY user, repo")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*UserRepoCount{}
	for rows.Next() {
		var (
			user  string
			repo  string
			count int
		)
		if err := rows.Scan(&user, &repo, &count); err != nil {
			log.Fatal(err)
		} else {
			item := new(UserRepoCount)
			item.user = user
			item.repo = repo
			item.count = count
			result = append(result, item)
		}
	}
	return result, nil
}

func (a *App) incrCount(user, repo string) (int, error) {
	count := 0
	err := a.db.QueryRow("SELECT count FROM counts WHERE user = ? AND repo = ?", user, repo).Scan(&count)

	switch {
	case err == sql.ErrNoRows:
		log.Printf("user/repo not in db: %s/%s\n", user, repo)
		_, err := a.db.Exec("INSERT INTO counts(user, repo, count) VALUES(?, ?, 1)", user, repo)
		if err != nil {
			return -1, err
		}
	case err != nil:
		return -1, err
	default:
		_, err := a.db.Exec("UPDATE counts SET count = count + 1 WHERE user = ? AND repo = ?", user, repo)
		if err != nil {
			return -1, err
		}
	}

	return count + 1, nil
}

func (a *App) createSVG(count int) string {
	template := `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="20">
<rect width="30" height="20" fill="#555"/>
<rect x="30" width="{recWidth}" height="20" fill="#4c1"/>
<rect rx="3" width="80" height="20" fill="transparent"/>
	<g fill="#fff" text-anchor="middle"
    font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
	    <text x="15" y="14">hits</text>
	    <text x="{textX}" y="14">{count}</text>
	</g>
</svg>`
	text := fmt.Sprintf("%d", count)
	length := len(text)
	width := 80
	recWidth := 50
	textX := 55
	if length >= 5 {
		width += 6 * (length - 5)
		recWidth += 6 * (length - 5)
		textX += 3 * (length - 5)
	}
	r := strings.NewReplacer("{count}", text,
		"{width}", fmt.Sprintf("%d", width),
		"{recWidth}", fmt.Sprintf("%d", recWidth),
		"{textX}", fmt.Sprintf("%d", textX))
	return r.Replace(template)
}
