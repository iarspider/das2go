/*
 *
 * Author     : Valentin Kuznetsov <vkuznet AT gmail dot com>
 * Description: this module contains all helper functions used in DAS services (Local APIs)
 * Created    : Fri Jun 26 14:25:01 EDT 2015
 *
 */
package services

import (
	"fmt"
	"github.com/vkuznet/das2go/dasql"
	"github.com/vkuznet/das2go/mongo"
	"github.com/vkuznet/das2go/utils"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type LocalAPIs struct{}

func dbsUrl(inst string) string {
	//     return "https://cmsweb.cern.ch/dbs/prod/global/DBSReader"
	return fmt.Sprintf("https://cmsweb.cern.ch/dbs/%s/DBSReader", inst)
}
func phedexUrl() string {
	return "https://cmsweb.cern.ch/phedex/datasvc/json/prod"
}
func sitedbUrl() string {
	return "https://cmsweb.cern.ch/sitedb/data/prod"
}

// Here I list __ONLY__ exceptional apis due to mistake in DAS maps
func DASLocalAPIs() []string {
	out := []string{
		// dbs3 APIs which should be treated as local_api, but they have
		// url: http://.... in their map instead of local_api
		"file_run_lumi4dataset", "file_run_lumi4block",
		"file_lumi4dataset", "file_lumi4block", "run_lumi4dataset", "run_lumi4block",
		"block_run_lumi4dataset", "file4dataset_run_lumi", "blocks4tier_dates",
		"lumi4block_run", "datasetlist"}
	return out
}

// helper function to find file,run,lumis for given dataset or block
func find_blocks(dasquery dasql.DASQuery) []string {
	spec := dasquery.Spec
	inst := dasquery.Instance
	var out []string
	blk := spec["block"]
	if blk != nil {
		out = append(out, blk.(string))
		return out
	}
	dataset := spec["dataset"].(string)
	api := "blocks"
	furl := fmt.Sprintf("%s/%s?dataset=%s", dbsUrl(inst), api, dataset)
	resp := utils.FetchResponse(furl, "") // "" specify optional args
	records := DBSUnmarshal(api, resp.Data)
	for _, rec := range records {
		out = append(out, rec["block_name"].(string))
	}
	return out
}

// helper function to process given set of urls and unmarshal results
// from all url calls
func processUrls(system, api string, urls []string) []mongo.DASRecord {
	var outRecords []mongo.DASRecord
	out := make(chan utils.ResponseType)
	umap := map[string]int{}
	for _, furl := range urls {
		umap[furl] = 1                // keep track of processed urls below
		go utils.Fetch(furl, "", out) // "" specify optional args
	}
	// collect all results from out channel
	exit := false
	for {
		select {
		case r := <-out:
			// process data
			var records []mongo.DASRecord
			if system == "dbs3" || system == "dbs" {
				records = DBSUnmarshal(api, r.Data)
			} else if system == "phedex" {
				records = PhedexUnmarshal(api, r.Data)
			}
			for _, rec := range records {
				rec["url"] = r.Url
				outRecords = append(outRecords, rec)
			}
			// remove from umap, indicate that we processed it
			delete(umap, r.Url) // remove Url from map
		default:
			if len(umap) == 0 { // no more requests, merge data records
				exit = true
			}
			time.Sleep(time.Duration(10) * time.Millisecond) // wait for response
		}
		if exit {
			break
		}
	}
	return outRecords
}

// helper function to get run arguments for given spec
// we extract run parameter from spec and construct run_num arguments for DBS
func runArgs(dasquery dasql.DASQuery) string {
	// get runs from spec
	spec := dasquery.Spec
	runs := spec["run"]
	runs_args := ""
	if runs != nil {
		switch value := runs.(type) {
		case []string:
			for _, val := range value {
				runs_args = fmt.Sprintf("%s&run_num=%s", runs_args, val)
			}
		case string:
			runs_args = fmt.Sprintf("%s&run_num=%s", runs_args, value)
		default:
			panic(fmt.Sprintf("Unknown type for runs=%s, type=%T", runs, runs))
		}
	}
	return runs_args
}

// helper function to get file status from the spec
func fileStatus(dasquery dasql.DASQuery) bool {
	spec := dasquery.Spec
	status := spec["status"]
	if status != nil {
		val := status.(string)
		if strings.ToLower(val) == "valid" {
			return true
		}
	}
	return false
}

// helper function to get DBS urls for given spec and api
func dbs_urls(dasquery dasql.DASQuery, api string) []string {
	inst := dasquery.Instance
	// get runs from spec
	runs_args := runArgs(dasquery)
	valid_file := fileStatus(dasquery)

	// find all blocks for given dataset or block
	var urls []string
	for _, blk := range find_blocks(dasquery) {
		myurl := fmt.Sprintf("%s/%s?block_name=%s", dbsUrl(inst), api, url.QueryEscape(blk))
		if len(runs_args) > 0 {
			myurl += runs_args // append run arguments
		}
		if valid_file {
			myurl += fmt.Sprintf("&validFileOnly=1") // append validFileOnly=1
		}
		urls = append(urls, myurl)
	}
	return utils.List2Set(urls)
}

// helper function to get file,run,lumi triplets
func file_run_lumi(dasquery dasql.DASQuery, keys []string) []mongo.DASRecord {
	var out []mongo.DASRecord

	// use filelumis DBS API output to get
	// run_num, logical_file_name, lumi_secion_num from provided fields
	api := "filelumis"
	urls := dbs_urls(dasquery, api)
	filelumis := processUrls("dbs3", api, urls)
	for _, rec := range filelumis {
		row := make(mongo.DASRecord)
		for _, key := range keys {
			// put into file das record, internal type must be list
			if key == "run_num" {
				row["run"] = []mongo.DASRecord{mongo.DASRecord{"run_number": rec[key]}}
			} else if key == "lumi_section_num" {
				row["lumi"] = []mongo.DASRecord{mongo.DASRecord{"number": rec[key]}}
			} else if key == "logical_file_name" {
				row["file"] = []mongo.DASRecord{mongo.DASRecord{"name": rec[key]}}
			}
		}
		out = append(out, row)
	}
	if len(keys) == 2 {
		if keys[0] == "run_num" && keys[1] == "lumi_section_num" {
			return OrderByRunLumis(out)
		} else if keys[0] == "lumi_section_num" && keys[1] == "run_num" {
			return OrderByRunLumis(out)
		}
	}
	return out
}

// helper function to sort records by run and then merge lumis within a run
func OrderByRunLumis(records []mongo.DASRecord) []mongo.DASRecord {
	var out []mongo.DASRecord
	rmap := make(map[float64][]float64)
	for _, r := range records {
		lumiList := mongo.GetValue(r, "lumi.number").([]interface{})
		run := mongo.GetValue(r, "run.run_number").(float64)
		lumis, ok := rmap[run]
		if ok {
			for _, v := range lumiList {
				lumis = append(lumis, v.(float64))
			}
			rmap[run] = lumis
		} else {
			var lumiValues []float64
			for _, v := range lumiList {
				lumiValues = append(lumiValues, v.(float64))
			}
			rmap[run] = lumiValues
		}
	}
	for run, lumis := range rmap {
		rec := mongo.DASRecord{"run": mongo.DASRecord{"run_number": run}, "lumi": mongo.DASRecord{"number": lumis}}
		out = append(out, rec)
	}
	return out
}

// helper function to get dataset for release
func dataset4release(dasquery dasql.DASQuery) []string {
	spec := dasquery.Spec
	inst := dasquery.Instance
	var out []string
	api := "datasets"
	release := spec["release"].(string)
	furl := fmt.Sprintf("%s/%s?release_version=%s", dbsUrl(inst), api, release)
	parent := spec["parent"]
	if parent != nil {
		furl = fmt.Sprintf("%s&parent_dataset=%s", furl, parent.(string))
	}
	status := spec["status"]
	if status != nil {
		furl = fmt.Sprintf("%s&dataset_access_type=%s", furl, status.(string))
	}
	resp := utils.FetchResponse(furl, "") // "" specify optional args
	records := DBSUnmarshal(api, resp.Data)
	for _, rec := range records {
		dataset := rec["name"].(string)
		if !utils.InList(dataset, out) {
			out = append(out, dataset)
		}
	}
	return out
}

// helper function to construct Phedex node API argument from given site
func phedexNode(site string) string {
	var node string
	nodeMatch, _ := regexp.MatchString("^T[0-9]_[A-Z]+(_)[A-Z]+", site)
	seMatch, _ := regexp.MatchString("^[a-z]+(\\.)[a-z]+(\\.)", site)
	if nodeMatch {
		node = fmt.Sprintf("node=%s", site)
		if !strings.HasSuffix(node, "*") {
			node += "*"
		}
	} else if seMatch {
		node = fmt.Sprintf("se=%s", site)
	} else {
		panic(fmt.Sprintf("ERROR: unable to match site name %s", site))
	}
	return node
}

// helper function to find datasets for given site and release
func dataset4site_release(dasquery dasql.DASQuery) []mongo.DASRecord {
	spec := dasquery.Spec
	var out []mongo.DASRecord
	var urls, datasets []string
	api := "blockReplicas"
	node := phedexNode(spec["site"].(string))
	for _, dataset := range dataset4release(dasquery) {
		furl := fmt.Sprintf("%s/%s?dataset=%s&%s", phedexUrl(), api, dataset, node)
		if !utils.InList(furl, urls) {
			urls = append(urls, furl)
		}
	}
	for _, rec := range processUrls("phedex", api, urls) {
		block := rec["name"].(string)
		dataset := strings.Split(block, "#")[0]
		if !utils.InList(dataset, datasets) {
			datasets = append(datasets, dataset)
		}
	}
	for _, name := range datasets {
		rec := make(mongo.DASRecord)
		row := make(mongo.DASRecord)
		row["name"] = name
		rec["dataset"] = []mongo.DASRecord{row}
		out = append(out, rec)
	}
	return out
}

// struct which cache PhEDEX nodes and periodically update them
type PhedexNodes struct {
	nodes  []mongo.DASRecord
	tstamp int64
}

// PhedexNodes API which periodically fetch PhEDEx nodes info
// if records still alive (fetched less than a day ago) we use the cache
func (p *PhedexNodes) Nodes() []mongo.DASRecord {
	if len(p.nodes) != 0 && (time.Now().Unix()-p.tstamp) < 24*60*60 {
		return p.nodes
	}
	api := "nodes"
	furl := fmt.Sprintf("%s/%s", phedexUrl(), api)
	resp := utils.FetchResponse(furl, "") // "" specify optional args
	p.nodes = PhedexUnmarshal(api, resp.Data)
	p.tstamp = time.Now().Unix()
	return p.nodes
}

// PhedexNodes API to return type of given node
func (p *PhedexNodes) NodeType(site string) string {
	nodeMatch, _ := regexp.MatchString("^T[0-9]_[A-Z]+(_)[A-Z]+", site)
	seMatch, _ := regexp.MatchString("^[a-z]+(\\.)[a-z]+(\\.)", site)
	nodes := p.Nodes()
	var siteName, seName, kind string
	for _, rec := range nodes {
		switch v := rec["se"].(type) {
		case string:
			seName = v
		default:
			seName = ""
		}
		switch v := rec["name"].(type) {
		case string:
			siteName = v
		default:
			siteName = ""
		}
		switch v := rec["kind"].(type) {
		case string:
			kind = v
		default:
			kind = ""
		}
		if nodeMatch && siteName == site {
			return kind
		} else if seMatch && seName == site {
			return kind
		}
	}
	return ""
}