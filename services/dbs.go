/*
 *
 * Author     : Valentin Kuznetsov <vkuznet AT gmail dot com>
 * Description: DBS module
 * Created    : Fri Jun 26 14:25:01 EDT 2015
 *
 */
package services

import (
	"encoding/json"
	"fmt"
	"github.com/vkuznet/das2go/dasql"
	"github.com/vkuznet/das2go/mongo"
	"github.com/vkuznet/das2go/utils"
	"log"
	"net/url"
	"strconv"
	"strings"
)

// helper function to load DBS data stream
func loadDBSData(api string, data []byte) []mongo.DASRecord {
	var out []mongo.DASRecord
	err := json.Unmarshal(data, &out)
	if err != nil {
		msg := fmt.Sprintf("DBS unable to unmarshal the data into DAS record, api=%s, data=%s, error=%v", api, string(data), err)
		panic(msg)
	}
	return out
}

// Unmarshal DBS data stream and return DAS records based on api
func DBSUnmarshal(api string, data []byte) []mongo.DASRecord {
	records := loadDBSData(api, data)
	var out []mongo.DASRecord
	if api == "dataset_info" || api == "datasets" || api == "datasetlist" {
		for _, rec := range records {
			rec["name"] = rec["dataset"]
			delete(rec, "dataset")
			out = append(out, rec)
		}
		return out
	} else if api == "physicsgroup" {
		for _, rec := range records {
			rec["name"] = rec["physics_group_name"]
			delete(rec, "physics_group_name")
			out = append(out, rec)
		}
		return out
	} else if api == "site4dataset" {
		for _, rec := range records {
			r := mongo.DASRecord{"name": rec["origin_site_name"], "dataset": rec["dataset"]}
			out = append(out, r)
		}
		return out
	} else if api == "fileparents" {
		for _, rec := range records {
			for _, v := range rec["parent_logical_file_name"].([]interface{}) {
				r := make(mongo.DASRecord)
				r["name"] = v.(string)
				out = append(out, r)
			}
		}
		return out
	} else if api == "runs_via_dataset" || api == "runs" {
		for _, rec := range records {
			for _, v := range rec["run_num"].([]interface{}) {
				r := make(mongo.DASRecord)
				r["run_number"] = fmt.Sprintf("%d", int(v.(float64)))
				out = append(out, r)
			}
		}
		return out
	}
	return records
}

/*
 * Local DBS3 APIs
 */

// dataset4block
func (LocalAPIs) L_dbs3_dataset4block(dasquery dasql.DASQuery) []mongo.DASRecord {
	spec := dasquery.Spec
	block := spec["block"].(string)
	dataset := strings.Split(block, "#")[0]
	var out []mongo.DASRecord
	rec := make(mongo.DASRecord)
	row := make(mongo.DASRecord)
	row["name"] = dataset
	rec["dataset"] = []mongo.DASRecord{row}
	out = append(out, rec)
	return out
}

func (LocalAPIs) L_dbs3_run_lumi4dataset(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"run_num", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}
func (LocalAPIs) L_dbs3_run_lumi4block(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"run_num", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}

func (LocalAPIs) L_dbs3_file_lumi4dataset(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"logical_file_name", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}
func (LocalAPIs) L_dbs3_file_lumi4block(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"logical_file_name", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}

func (LocalAPIs) L_dbs3_file_run_lumi4dataset(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"logical_file_name", "run_num", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}
func (LocalAPIs) L_dbs3_file_run_lumi4block(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"logical_file_name", "run_num", "lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}
func (LocalAPIs) L_dbs3_block_run_lumi4dataset(dasquery dasql.DASQuery) []mongo.DASRecord {
	var out []mongo.DASRecord
	keys := []string{"block_name", "run_num", "lumi_section_num"}
	// use filelumis DBS API output to get
	// run_num, logical_file_name, lumi_secion_num from provided keys
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
			} else if key == "block_name" {
				rurl, err := url.QueryUnescape(rec["url"].(string))
				if err != nil {
					log.Println("DAS ERROR, unable to parse url", rec)
					panic(err)
				}
				arr := strings.Split(rurl, "block_name=")
				blk := arr[1]
				row["block"] = []mongo.DASRecord{mongo.DASRecord{"name": blk}}
			}
		}
		out = append(out, row)
	}
	return out
}
func (LocalAPIs) L_dbs3_file4dataset_run_lumi(dasquery dasql.DASQuery) []mongo.DASRecord {
	spec := dasquery.Spec
	var out []mongo.DASRecord
	lumi, _ := strconv.ParseFloat(spec["lumi"].(string), 64)
	keys := []string{"logical_file_name", "lumi_section_num"}
	records := file_run_lumi(dasquery, keys)
	for _, rec := range records {
		for _, row := range rec["lumi"].([]mongo.DASRecord) {
			lumis := row["number"].([]interface{})
			for _, val := range lumis {
				if lumi == val.(float64) {
					row := make(mongo.DASRecord)
					row["file"] = rec["file"]
					out = append(out, row)
				}
			}
		}
	}
	return out
}

func (LocalAPIs) L_dbs3_blocks4tier_dates(dasquery dasql.DASQuery) []mongo.DASRecord {
	spec := dasquery.Spec
	inst := dasquery.Instance
	var out []mongo.DASRecord
	tier := spec["tier"].(string)
	dates := spec["date"].([]string)
	mind := utils.UnixTime(dates[0])
	maxd := utils.UnixTime(dates[1])
	api := "blocks"
	furl := fmt.Sprintf("%s/%s?data_tier_name=%s&min_cdate=%d&max_cdate=%d", dbsUrl(inst), api, tier, mind, maxd)
	log.Println(furl)
	resp := utils.FetchResponse(furl, "") // "" specify optional args
	records := DBSUnmarshal(api, resp.Data)
	var blocks []string
	for _, rec := range records {
		blk := rec["block_name"].(string)
		dataset := strings.Split(blk, "#")[0]
		tierName := strings.Split(dataset, "/")[3]
		if tierName == tier && !utils.InList(blk, blocks) {
			blocks = append(blocks, blk)
		}
	}
	for _, name := range blocks {
		rec := make(mongo.DASRecord)
		row := make(mongo.DASRecord)
		row["name"] = name
		rec["block"] = []mongo.DASRecord{row}
		out = append(out, rec)
	}
	return out
}
func (LocalAPIs) L_dbs3_lumi4block_run(dasquery dasql.DASQuery) []mongo.DASRecord {
	keys := []string{"lumi_section_num"}
	return file_run_lumi(dasquery, keys)
}

func (LocalAPIs) L_dbs3_datasetlist(dasquery dasql.DASQuery) []mongo.DASRecord {
	spec := dasquery.Spec
	inst := dasquery.Instance
	api := "datasetlist"
	furl := fmt.Sprintf("%s/%s", dbsUrl(inst), api)
	args, err := json.Marshal(spec)
	if err != nil {
		msg := fmt.Sprintf("DBS datasetlist unable to marshal the spec %v, error %v", spec, err)
		panic(msg)
	}
	log.Println(furl, string(args))
	resp := utils.FetchResponse(furl, string(args)) // POST request
	records := DBSUnmarshal(api, resp.Data)
	return records
}