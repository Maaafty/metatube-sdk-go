package engine

import (
	"sort"
	"sync"

	"gorm.io/gorm/clause"

	"github.com/javtube/javtube-sdk-go/model"
	javtube "github.com/javtube/javtube-sdk-go/provider"
	"github.com/javtube/javtube-sdk-go/provider/gfriends"
)

func (e *Engine) searchActor(keyword string, provider javtube.Provider, lazy bool) (results []*model.ActorSearchResult, err error) {
	if provider.Name() == gfriends.Name {
		return provider.(javtube.ActorSearcher).SearchActor(keyword)
	}
	if searcher, ok := provider.(javtube.ActorSearcher); ok {
		// Query DB first (by name or id).
		if info := new(model.ActorInfo); lazy {
			if result := e.db.
				Where("provider = ?", provider.Name()).
				Where(e.db.
					Where("name = ?", keyword).
					Or("id = ?", keyword)).
				First(info); result.Error == nil && info.Valid() /* must be valid */ {
				return []*model.ActorSearchResult{info.ToSearchResult()}, nil
			}
		}
		return searcher.SearchActor(keyword)
	}
	// All providers should implement ActorSearcher interface.
	return nil, javtube.ErrInfoNotFound
}

func (e *Engine) SearchActor(keyword, name string, lazy bool) ([]*model.ActorSearchResult, error) {
	provider, err := e.GetActorProviderByName(name)
	if err != nil {
		return nil, err
	}
	return e.searchActor(keyword, provider, lazy)
}

func (e *Engine) SearchActorAll(keyword string) (results []*model.ActorSearchResult, err error) {
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, provider := range e.actorProviders {
		wg.Add(1)
		go func(provider javtube.ActorProvider) {
			defer wg.Done()
			if innerResults, innerErr := e.searchActor(keyword, provider, true); innerErr == nil {
				for _, result := range innerResults {
					if result.Valid() /* validation check */ {
						mu.Lock()
						results = append(results, result)
						mu.Unlock()
					}
				}
			} // ignore error
		}(provider)
	}
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool {
		return e.MustGetActorProviderByName(results[i].Provider).Priority() >
			e.MustGetActorProviderByName(results[j].Provider).Priority()
	})
	return
}

func (e *Engine) getActorInfoFromDB(provider javtube.ActorProvider, id string) (*model.ActorInfo, error) {
	info := &model.ActorInfo{}
	err := e.db. // Exact match here.
			Where("provider = ?", provider.Name()).
			Where("id = ?", id).
			First(info).Error
	return info, err
}

func (e *Engine) getActorInfoWithCallback(provider javtube.ActorProvider, id string, lazy bool, callback func() (*model.ActorInfo, error)) (info *model.ActorInfo, err error) {
	defer func() {
		// metadata validation check.
		if err == nil && (info == nil || !info.Valid()) {
			err = javtube.ErrIncompleteMetadata
		}
	}()
	if provider.Name() == gfriends.Name {
		return provider.GetActorInfoByID(id)
	}
	// Query DB first (by id).
	if lazy {
		if info, err = e.getActorInfoFromDB(provider, id); err == nil && info.Valid() {
			return
		}
	}
	// Delayed info auto-save.
	defer func() {
		if err == nil && info.Valid() {
			// Make sure we save the original info here.
			e.db.Clauses(clause.OnConflict{
				UpdateAll: true,
			}).Create(info) // ignore error
		}
	}()
	return callback()
}

func (e *Engine) getActorInfoByProviderID(provider javtube.ActorProvider, id string, lazy bool) (*model.ActorInfo, error) {
	if id = provider.NormalizeID(id); id == "" {
		return nil, javtube.ErrInvalidID
	}
	return e.getActorInfoWithCallback(provider, id, lazy, func() (*model.ActorInfo, error) {
		return provider.GetActorInfoByID(id)
	})
}

func (e *Engine) GetActorInfoByProviderID(name, id string, lazy bool) (*model.ActorInfo, error) {
	provider, err := e.GetActorProviderByName(name)
	if err != nil {
		return nil, err
	}
	return e.getActorInfoByProviderID(provider, id, lazy)
}

func (e *Engine) getActorInfoByProviderURL(provider javtube.ActorProvider, rawURL string, lazy bool) (*model.ActorInfo, error) {
	id, err := provider.ParseIDFromURL(rawURL)
	switch {
	case err != nil:
		return nil, err
	case id == "":
		return nil, javtube.ErrInvalidURL
	}
	return e.getActorInfoWithCallback(provider, id, lazy, func() (*model.ActorInfo, error) {
		return provider.GetActorInfoByURL(rawURL)
	})
}

func (e *Engine) GetActorInfoByURL(rawURL string, lazy bool) (*model.ActorInfo, error) {
	provider, err := e.GetActorProviderByURL(rawURL)
	if err != nil {
		return nil, err
	}
	return e.getActorInfoByProviderURL(provider, rawURL, lazy)
}
