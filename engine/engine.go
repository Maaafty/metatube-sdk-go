package engine

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/javtube/javtube-sdk-go/common/fetch"
	"github.com/javtube/javtube-sdk-go/model"
	javtube "github.com/javtube/javtube-sdk-go/provider"
)

type Engine struct {
	db *gorm.DB
	// Name:Provider Map
	actorProviders map[string]javtube.ActorProvider
	movieProviders map[string]javtube.MovieProvider
	// Host:Providers Map
	actorHostProviders map[string][]javtube.ActorProvider
	movieHostProviders map[string][]javtube.MovieProvider
}

func New(db *gorm.DB, timeout time.Duration) *Engine {
	engine := &Engine{db: db}
	engine.initActorProviders(timeout)
	engine.initMovieProviders(timeout)
	return engine
}

// initActorProviders initializes actor providers.
func (e *Engine) initActorProviders(timeout time.Duration) {
	{ // init
		e.actorProviders = make(map[string]javtube.ActorProvider)
		e.actorHostProviders = make(map[string][]javtube.ActorProvider)
	}
	javtube.RangeActorFactory(func(name string, factory javtube.ActorFactory) {
		provider := factory()
		if s, ok := provider.(javtube.RequestTimeoutSetter); ok {
			s.SetRequestTimeout(timeout)
		}
		// Add actor provider by name.
		e.actorProviders[strings.ToUpper(name)] = provider
		// Add actor provider by host.
		host := provider.URL().Hostname()
		e.actorHostProviders[host] = append(e.actorHostProviders[host], provider)
	})
}

// initMovieProviders initializes movie providers.
func (e *Engine) initMovieProviders(timeout time.Duration) {
	{ // init
		e.movieProviders = make(map[string]javtube.MovieProvider)
		e.movieHostProviders = make(map[string][]javtube.MovieProvider)
	}
	javtube.RangeMovieFactory(func(name string, factory javtube.MovieFactory) {
		provider := factory()
		if s, ok := provider.(javtube.RequestTimeoutSetter); ok {
			s.SetRequestTimeout(timeout)
		}
		// Add movie provider by name.
		e.movieProviders[strings.ToUpper(name)] = provider
		// Add movie provider by host.
		host := provider.URL().Hostname()
		e.movieHostProviders[host] = append(e.movieHostProviders[host], provider)
	})
}

func (e *Engine) IsActorProvider(name string) (ok bool) {
	_, ok = e.actorProviders[strings.ToUpper(name)]
	return
}

func (e *Engine) GetActorProviderByURL(rawURL string) (javtube.ActorProvider, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	for _, p := range e.actorHostProviders[u.Hostname()] {
		if strings.HasPrefix(u.Path, p.URL().Path) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("actor provider not found: %s", rawURL)
}

func (e *Engine) GetActorProviderByName(name string) (javtube.ActorProvider, error) {
	provider, ok := e.actorProviders[strings.ToUpper(name)]
	if !ok {
		return nil, fmt.Errorf("actor provider not found: %s", name)
	}
	return provider, nil
}

func (e *Engine) MustGetActorProviderByName(name string) javtube.ActorProvider {
	provider, err := e.GetActorProviderByName(name)
	if err != nil {
		panic(err)
	}
	return provider
}

func (e *Engine) IsMovieProvider(name string) (ok bool) {
	_, ok = e.movieProviders[strings.ToUpper(name)]
	return
}

func (e *Engine) GetMovieProviderByURL(rawURL string) (javtube.MovieProvider, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	for _, p := range e.movieHostProviders[u.Hostname()] {
		if strings.HasPrefix(u.Path, p.URL().Path) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("movie provider not found: %s", rawURL)
}

func (e *Engine) GetMovieProviderByName(name string) (javtube.MovieProvider, error) {
	provider, ok := e.movieProviders[strings.ToUpper(name)]
	if !ok {
		return nil, fmt.Errorf("movie provider not found: %s", name)
	}
	return provider, nil
}

func (e *Engine) MustGetMovieProviderByName(name string) javtube.MovieProvider {
	provider, err := e.GetMovieProviderByName(name)
	if err != nil {
		panic(err)
	}
	return provider
}

func (e *Engine) AutoMigrate(v bool) error {
	if !v {
		return nil
	}
	return e.db.AutoMigrate(
		&model.MovieInfo{},
		&model.ActorInfo{})
}

func (e *Engine) Fetch(url string, provider javtube.Provider) (*http.Response, error) {
	// Provider which implements Fetcher interface should be
	// used to fetch all its corresponding resources.
	if fetcher, ok := provider.(javtube.Fetcher); ok {
		return fetcher.Fetch(url)
	}
	return fetch.Fetch(url)
}
