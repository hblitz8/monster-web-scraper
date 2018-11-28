/*
	Request format:

	POST /get_jobs HTTP/1.0
	Content-Type: application/json

	[
		“http://www.indeed.com/viewjob?jk=8cfd54301d909668”,
		“http://www.indeed.com/viewjob?jk=b17c354e3cabe4f1”,
		“http://www.indeed.com/viewjob?jk=38123d02e67210d9”,
		…
	]
/*
/*

Response format:

	HTTP/1.1 200 OK
	Date: Wed, 17 May 2016 01:45:49 GMT
	Content-Type: application/json

	[
		{
			“title”: “Software Engineer”,
			“location”: “San Francisco, CA”,
			“company”: “MuleSoft”,
			“url”: “http://www.indeed.com/viewjob?jk=8cfd54301d909668”
		},
		{
			“title”: “<job title>”,
			“location”: “<job location>”,
			“company”: “<company name>”,
			“url”: “<original url>”
		},
		…
]
*/

package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"sync"
)

type response struct {
	Title    string `json:"title"`
	Location string `json:"location"`
	Company  string `json:"company"`
	Url      string `json:"url"`
}

type SafeResponseArray struct {
	array []response
	mux   sync.Mutex
}

// Extract all needed fields from the page
func crawl(url string, results *SafeResponseArray) {

	resp, err := http.Get(url)

	if err != nil {
		fmt.Println("ERROR: Failed to crawl \"" + url + "\"")
		return
	}

	b := resp.Body
	defer b.Close() // close Body when the function returns

	if resp.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(b)
	if err != nil {
		log.Fatal(err)
	}

	res := response{}
	doc.Find(".jobsearch-JobInfoHeader-title").Each(func(i int, s *goquery.Selection) {
		res.Title = s.Text()
	})

	doc.Find(".jobsearch-InlineCompanyRating > div:last-child").Each(func(i int, s *goquery.Selection) {
		// assume the job location is the last div inside .jobsearch-InlineCompanyRating
		res.Location = s.Text()
	})

	doc.Find(".jobsearch-CompanyAvatar-companyLink").Each(func(i int, s *goquery.Selection) {
		res.Company= s.Text()
	})
	res.Url = url

	results.mux.Lock()
	results.array = append(results.array, res)
	results.mux.Unlock()

}

func worker_crawl(id int, jobs <-chan string, results *SafeResponseArray, wg *sync.WaitGroup) {

	for {
		url, more := <-jobs
		if more {
			// fmt.Printf("worker #%d received job %s\n", id, url)
			crawl(url, results)
		} else {
			// fmt.Println("received all jobs")
			defer wg.Done()
			return
		}
	}

}

func handler_get_jobs(w http.ResponseWriter, r *http.Request) {

	var results SafeResponseArray
	wg := new(sync.WaitGroup)

	//seedUrls := os.Args[1:]

	var seedUrls []string
	json.NewDecoder(r.Body).Decode(&seedUrls)

	jobs := make(chan string)

	// Kick off the crawl process concurrently
	for i := 1; i <= 10; i++ {
		// 10 threads max
		wg.Add(1)
		go worker_crawl(i, jobs, &results, wg)
	}

	for _, url := range seedUrls {
		jobs <- url
	}
	close(jobs)

	wg.Wait()

	b, _ := json.Marshal(results.array)
	output := string(b)

	// log.Printf(output)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", output)

}

func handler(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path
	if r.Method == "POST" && path == "/get_jobs" {
		// assume case sensitive path for "/get_jobs"
		handler_get_jobs(w, r)
	}

}

func main() {

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":8080", nil))

}
