package IM

import (
	"crypto/md5"
	"time"
	"encoding/hex"
	"fmt"
	"sync"
	"errors"
	"strings"
)

/*
* A file proxy can automatically cache file and generate file from byte stream
* then return an url for html to fetch the file
*/
type FileProxy struct {
	mutex sync.Mutex
	files map[string]ProxyFile

	host string
	proxyRoot string
}

/*
* Delete a file from file proxy
*/
func (p *FileProxy) DeleteFile(url string) {
	p.mutex.Lock()
	delete(p.files, url)
	p.mutex.Unlock()
}

/*
* Fetch content from file proxy, if file not found, return error
*/
func (p *FileProxy) FetchFile(hash string) ([]byte, error) {
	if f, ok := p.files[hash]; ok {
		return f.Content(), nil
	} else {
		return nil, errors.New("No such file: " + hash)
	}
}

/*
* Add a disposable file which can be fetch only once
* Return value is the url for the file so that html can fetch the file by it
*/
func (p *FileProxy) AddDisposableFile(file []byte, suffix string) string {
	src := []byte(time.Now().String() + suffix)

	h := md5.New()
	h.Write(src)
	hash := hex.EncodeToString(h.Sum(nil))
	var url string
	if suffix != "" {
		url = fmt.Sprintf("%s/%s.%s", hash, hash, suffix)
	} else {
		url = fmt.Sprintf("%s/%s", hash, hash)
	}

	fmt.Println("hash:", hash)
	p.mutex.Lock()
	p.files[hash] = NewCacheFile(file, hash, p)
	p.mutex.Unlock()

	return url
}

func (p *FileProxy) AddDisposalFileWithRestfulAPI(file []byte, suffix string) string {
	return fmt.Sprintf("%s/%s/%s", p.host, p.proxyRoot, p.AddDisposableFile(file, suffix))
}

func (p *FileProxy) AddDisposalNamedFile(file []byte, filename string) string {
	src := []byte(time.Now().String() + filename)

	h := md5.New()
	h.Write(src)
	hash := hex.EncodeToString(h.Sum(nil))
	url := fmt.Sprintf("%s/%s", hash, filename)

	p.mutex.Lock()
	p.files[hash] = NewCacheNamedFile(file, hash, filename, p)
	p.mutex.Unlock()

	return url
}

func (p FileProxy) AddDisposalNamedFileWithRestfulAPI(file []byte, filename string) string {
	return fmt.Sprintf("%s/%s/%s", p.host, p.proxyRoot, p.AddDisposableFile(file, filename))
}

/*
* Create a new file proxy with a root path
*/
func NewFileProxy(rootPath string, host string) *FileProxy {
	if strings.HasSuffix(rootPath, "/") {
		rootPath = rootPath[:len(rootPath) - 2]
	}
	if strings.HasPrefix(rootPath, "/") {
		rootPath = rootPath[1:]
	}
	if strings.HasSuffix(host, "/") {
		host = host[:len(rootPath) - 2]
	}
	if strings.HasPrefix(host, "/") {
		host = host[1:]
	}

	return &FileProxy{
		files			: make(map[string]ProxyFile),
		host			: host,
		proxyRoot		: rootPath,
	}
}

/*
* An interface of proxy files
*/
type ProxyFile interface {
	Content() []byte
	Expire()
}

/*
* Cached File will just stay in memory with out any persistent operation
*/
type CachedFile struct {
	proxy *FileProxy
	content []byte
	hash string
}
func (f *CachedFile) Content() []byte {
	f.Expire()
	return f.content
}
func (f *CachedFile) Expire() {
	f.proxy.DeleteFile(f.hash)
}
func NewCacheFile(content []byte, hash string, proxy *FileProxy) *CachedFile {
	return &CachedFile{
		content		: content,
		proxy		: proxy,
		hash		: hash,
	}
}

/*
* A cached named file
*/
type CacheNamedFile struct {
	CachedFile
	name string
}

func NewCacheNamedFile(content []byte, hash string, name string, proxy *FileProxy) *CacheNamedFile {
	return &CacheNamedFile{
		CachedFile : CachedFile{
			content		: content,
			proxy		: proxy,
			hash		: hash,
		},

		name		: name,
	}
}
