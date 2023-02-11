package src

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type Post struct {
	Title    string
	Content  template.HTML
	Date     time.Time
	Picture  string
	Tags     []string
	URLTitle string
}

type Posts []Post

func NewPost(fileContent io.Reader) (Post, error) {
	post, err := getPost(fileContent)
	if err != nil {
		return Post{}, err
	}
	return post, nil
}

func getPost(r io.Reader) (Post, error) {
	post := Post{}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	readLine := func() string {
		scanner.Scan()
		return scanner.Text()
	}

	title := readLine()
	post.Title = title
	date, err := StringToDate(readLine())
	if err != nil {
		return Post{}, err
	}
	post.Date = date
	post.Picture = readLine()
	post.Tags = strings.Split(readLine(), ",")
	readLine()

	body := bytes.Buffer{}
	for scanner.Scan() {
		body.Write(scanner.Bytes())
		body.WriteString("\n")
	}
	post.Content = RenderMarkdown(body.Bytes())
	post.URLTitle = strings.Replace(title, " ", "-", -1)

	return post, nil
}

type Event struct {
	Title    string
	Content  template.HTML
	Date     time.Time
	Tags     []string
	Link     string
	URLTitle string
}

type Events []Event

func New(fileContent io.Reader) (Event, error) {
	event, err := getEvent(fileContent)
	if err != nil {
		return Event{}, err
	}

	return event, nil
}

func getEvent(r io.Reader) (Event, error) {
	post := Event{}
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)

	readLine := func() string {
		scanner.Scan()
		return scanner.Text()
	}
	title := readLine()
	post.Title = title
	date, err := StringToDate(readLine())
	if err != nil {
		return Event{}, err
	}
	post.Date = date
	post.Link = readLine()
	post.Tags = strings.Split(readLine(), ",")
	readLine()

	body := bytes.Buffer{}
	for scanner.Scan() {
		body.Write(scanner.Bytes())
		body.WriteString("\n")
	}
	post.Content = RenderMarkdown(body.Bytes())
	post.URLTitle = strings.Replace(title, " ", "-", -1)

	return post, nil
}

type PostService interface {
	GetPosts() []Post
	GetPost(title string) (Post, error)
}

type EventService interface {
	GetEvents() []Event
	GetEvent(title string) (Event, error)
}

type BlogHandler struct {
	template     *template.Template
	postService  PostService
	eventService EventService
}

func NewHandler(
	template *template.Template,
	eventService EventService,
	postService PostService,
) *BlogHandler {
	return &BlogHandler{
		template:     template,
		postService:  postService,
		eventService: eventService,
	}
}

func (s *BlogHandler) ViewHome(w http.ResponseWriter, _ *http.Request) {
	err := s.template.ExecuteTemplate(w, "home.gohtml", s.postService.GetPosts())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *BlogHandler) ViewBlogs(w http.ResponseWriter, _ *http.Request) {
	err := s.template.ExecuteTemplate(w, "blogs.gohtml", s.postService.GetPosts())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *BlogHandler) ViewPost(w http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	urlTitle := vars["URLTitle"]
	post, err := s.postService.GetPost(urlTitle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = s.template.ExecuteTemplate(w, "locluong.gohtml", post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *BlogHandler) ViewRandoms(w http.ResponseWriter, e *http.Request) {
	err := s.template.ExecuteTemplate(w, "random.gohtml", s.eventService.GetEvents())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *BlogHandler) ViewRandom(w http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	urlTitle := vars["URLTitle"]
	post, err := s.eventService.GetEvent(urlTitle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = s.template.ExecuteTemplate(w, "locluong.gohtml", post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// err := s.template.ExecuteTemplate(w, "random.gohtml", s.eventService.GetEvents())
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
}

// Server configuration
type ServerConfig struct {
	CSSDir           string
	HTMLDir          string
	Port             string
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	PostsDir         string
	EventsDir        string
}

func (c ServerConfig) TCPAddress() string {
	return ":" + c.Port
}

func newRouter(handler *BlogHandler, cssFolderPath string) *mux.Router {
	router := mux.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Cache-Control", "public, max-age=86400")
			next.ServeHTTP(w, r)
		})
	})
	router.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.Dir(cssFolderPath))))
	router.HandleFunc("/", handler.ViewHome).Methods(http.MethodGet)
	router.HandleFunc("/viewBlogs", handler.ViewBlogs).Methods(http.MethodGet)
	router.HandleFunc("/blog/{URLTitle}", handler.ViewPost).Methods(http.MethodGet)
	router.HandleFunc("/randomThoughts", handler.ViewRandoms).Methods(http.MethodGet)
	router.HandleFunc("/random/{URLTitle}", handler.ViewRandom).Methods(http.MethodGet)
	router.NotFoundHandler = router.NewRoute().HandlerFunc(http.NotFound).GetHandler()

	return router
}

func NewServer(config ServerConfig, handler *BlogHandler) (server *http.Server) {
	router := newRouter(handler, config.CSSDir)

	server = &http.Server{
		Addr:         config.TCPAddress(),
		Handler:      router,
		ReadTimeout:  config.HTTPReadTimeout,
		WriteTimeout: config.HTTPWriteTimeout,
	}

	return
}

// File system to update posts and events
func NewPosts(postsDir fs.FS) (Posts, error) {
	dir, err := fs.ReadDir(postsDir, ".")
	if err != nil {
		return nil, fmt.Errorf("cannot read the posts directory: %s", err)
	}
	return getSortedPosts(postsDir, dir)
}

func getSortedPosts(postsDir fs.FS, dir []fs.DirEntry) ([]Post, error) {
	var posts []Post
	for _, file := range dir {
		post, err := newPostFromFile(postsDir, file.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot create a new post: %s", err)
		}
		posts = append(posts, post)
	}

	return sortByDate(posts), nil
}

func newPostFromFile(postsDir fs.FS, fileName string) (Post, error) {
	f, err := postsDir.Open(fileName)
	if f == nil {
		return Post{}, errors.New("this should not be a nil pointer")
	}

	defer f.Close()

	if err != nil {
		return Post{}, fmt.Errorf("cannot open the file %s: %s", fileName, err)
	}

	return NewPost(f)
}

func sortByDate(posts []Post) []Post {
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts
}

func NewEvents(eventsDir fs.FS) (Events, error) {
	dir, err := fs.ReadDir(eventsDir, ".")
	if err != nil {
		return nil, fmt.Errorf("cannot read the events directory: %s", err)
	}
	return getSortedEvents(eventsDir, dir)
}

func getSortedEvents(eventsDir fs.FS, dir []fs.DirEntry) ([]Event, error) {
	var events []Event
	for _, file := range dir {
		event, err := newEventFromFile(eventsDir, file.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot create a new event: %s", err)
		}
		events = append(events, event)
	}

	return sortEventsByDate(events), nil
}

func sortEventsByDate(events []Event) []Event {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date.After(events[j].Date)
	})
	return events
}

func newEventFromFile(eventsDir fs.FS, fileName string) (Event, error) {
	f, err := eventsDir.Open(fileName)
	if f == nil {
		return Event{}, errors.New("this should not be a nil pointer")
	}

	defer f.Close()

	if err != nil {
		return Event{}, fmt.Errorf("cannot open the file %s: %s", fileName, err)
	}

	return New(f)
}

type EventStore struct {
	events []Event
}

func NewEventStore(eventsDir fs.FS) (*EventStore, error) {
	events, err1 := NewEvents(eventsDir)
	if err1 != nil {
		return nil, err1
	}

	return &EventStore{events: events}, nil
}

func (i *EventStore) GetEvents() []Event {
	return i.events
}

func (i *EventStore) GetEvent(urlTitle string) (Event, error) {
	for _, event := range i.events {
		if event.URLTitle == urlTitle {
			return event, nil
		}
	}

	return Event{}, errors.New("blog not found")
}

type PostSore struct {
	posts []Post
}

func NewPostStore(postsDir fs.FS) (*PostSore, error) {
	posts, err := NewPosts(postsDir)
	if err != nil {
		return nil, err
	}

	return &PostSore{posts: posts}, nil
}

func (i *PostSore) GetPost(urlTitle string) (Post, error) {
	for _, post := range i.posts {
		if post.URLTitle == urlTitle {
			return post, nil
		}
	}

	return Post{}, errors.New("blog not found")
}

func (i *PostSore) GetPosts() []Post {
	return i.posts
}

// Main application
type App struct {
	Config  ServerConfig
	Handler BlogHandler
}

func NewApplication(config ServerConfig) (*App, error) {
	eventStore, err := NewEventStore(os.DirFS(config.EventsDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create the event store: %s", err)
	}

	postStore, err := NewPostStore(os.DirFS(config.PostsDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create the post store: %s", err)
	}

	templ, err := newTemplate(config.HTMLDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create templates: %s", err)
	}

	handler := NewHandler(templ, eventStore, postStore)

	return &App{
		Config:  config,
		Handler: *handler,
	}, nil
}

func newTemplate(tempFolderPath string) (*template.Template, error) {
	temp, err := template.ParseGlob(tempFolderPath)
	if err != nil {
		return nil, fmt.Errorf(
			"could not load template from %q, %v", tempFolderPath, err,
		)
	}
	return temp, nil
}

func NewConfig() ServerConfig {
	return ServerConfig{
		Port:             lookupEnvOr("PORT", defaultPort),
		HTTPReadTimeout:  defaultHTTPReadTimeout,
		HTTPWriteTimeout: defaulHTTPtWriteTimeout,
		CSSDir:           defaultCSSDir,
		HTMLDir:          defaultHTMLDir,
		PostsDir:         "public/posts",
		EventsDir:        "public/events",
	}
}

const (
	defaultCSSDir           = "./css"
	defaultHTMLDir          = "./html/*"
	defaultHTTPReadTimeout  = 10 * time.Second
	defaulHTTPtWriteTimeout = 10 * time.Second
	defaultPort             = "5000"
)

func lookupEnvOr(key string, defaultValue string) string {
	port, ok := os.LookupEnv(key)
	if !ok {
		port = defaultValue
	}
	return port
}
