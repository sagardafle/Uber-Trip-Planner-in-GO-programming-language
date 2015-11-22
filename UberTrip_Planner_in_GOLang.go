package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

/*type temp struct {
	destids []bson.ObjectId
}*/

//Userinput Struct of json string that will collwct data from the POSTMAN/CURL
type Userinput struct {
	Starting_from_location_id string   `json:"starting_from_location_id"`
	Location_ids              []string `json:"location_ids"`
	//	LocationIds               string `json:"locationIds"`
}

/*type Intermediate struct {
	Visited Location_ids `json:"visited"`
}*/

var t Userinput
var output []string

//var output t.Location_ids

//Geometry struct will have only lattitude and longitude values.
type Geometry struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

/**
ResponseBody struct have a nested Geometry struct created above. This struct will be used whenever the response is to be sent back to the user via POSTMAN/CURL.
Usually used in POST,GET,UPDATE methods.
**/
type ResponseBody struct {
	ID         bson.ObjectId `json:"id" bson:"_id"`
	Name       string        `json:"name"`
	Address    string        `json:"address"`
	City       string        `json:"city"`
	State      string        `json:"state"`
	Zip        string        `json:"zip"`
	Coordinate Geometry      `json:"coordinate"`
}

var m ResponseBody

type ResponseforPost struct {
	ID                        bson.ObjectId `json:"id" bson:"_id"`
	Status                    string        `json:"status"`
	Starting_from_location_id string        `json:"starting_from_location_id"`
	Best_route_location_ids   []string      `json:"best_route_location_ids"`
	Total_uber_costs          int           `json:"total_uber_costs"`
	Total_uber_duration       int           `json:"total_uber_duration"`
	Total_distance            float64       `json:"total_distance"`
}

var rfp ResponseforPost //Refrence for the ResponseforPost struct

var (
	mgoSession *mgo.Session
)

type UberAPI struct {
	Prices []struct {
		DisplayName  string  `json:"display_name"`
		Distance     float64 `json:"distance"`
		Duration     int     `json:"duration"`
		Estimate     string  `json:"estimate"`
		HighEstimate int     `json:"high_estimate"`
		LowEstimate  int     `json:"low_estimate"`
		Minimum      int     `json:"minimum"`
		ProductID    string  `json:"product_id"`
	} `json:"prices"`
}

type PutStruct struct {
	trip_route       []string
	node_visited_map map[string]int
}

type TripPutOutput struct {
	Id                           bson.ObjectId `json:"_id" bson:"_id,omitempty"`
	Status                       string        `json:"status"`
	Starting_from_location_id    string        `json:"starting_from_location_id"`
	Next_destination_location_id string        `json:"next_destination_location_id"`
	Best_route_location_ids      []string
	Total_uber_costs             int     `json:"total_uber_costs"`
	Total_uber_duration          int     `json:"total_uber_duration"`
	Total_distance               float64 `json:"total_distance"`
	Uber_wait_time_eta           int     `json:"uber_wait_time_eta"`
}

type PutStructForResponse struct {
	finalMap map[string]PutStruct
}

type Interim_data struct {
	Id               string   `json:"_id" bson:"_id,omitempty"`
	Trip_visited     []string `json:"node_visited_list"`
	Trip_not_visited []string `json:"trip_not_visited"`
	Trip_completed   int      `json:"trip_completed"`
}

type UberETAEstimateStruct struct {
	Request_id      string  `json:"request_id"`
	Status          string  `json:"status"`
	Vehicle         string  `json:"vehicle"`
	Driver          string  `json:"driver"`
	Location        string  `json:"location"`
	ETA             int     `json:"eta"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
}

var initialstartingPointOID bson.ObjectId
var startingPointOID bson.ObjectId
var priceestimatestruct UberAPI
var visited = make(map[int]bool)
var cheapestCostArray, durationArray []int
var distanceArray []float64
var totalcost, totalduration int
var totaldistance float64
var input []string
var isVisited = make([]bool, 15)
var startingPointVisited bool

func determineStartAndEndPoints(oid bson.ObjectId, input []string, output []string, rw http.ResponseWriter) {
	start_latitude, start_longitude := getcoordinatesfromdatabase(oid)

	for index := 0; index < len(input); index++ {

		if !visited[index] && len(input) > 0 {
			destinationID := bson.ObjectIdHex(input[index])

			end_Latitude, end_Longitude := getcoordinatesfromdatabase(destinationID)
			priceestimateapi := "https://sandbox-api.uber.com/v1/estimates/price?start_latitude=<start_latitude>&start_longitude=<start_longitude>&end_latitude=<end_latitude>&end_longitude=<end_longitude>&server_token=j6SYjh7dD1Q5E8MSJYvh4FQ4N_JdEu8PuBkNDhPQ"
			priceestimatestring := generatePriceEstimateURL(priceestimateapi, start_latitude, start_longitude, end_Latitude, end_Longitude)
			priceestimatestruct := getPriceEstimateAPIresults(priceestimatestring)

			cheapestCostArray, durationArray, distanceArray = calculateEstimatesBetweenNodes(priceestimatestruct, index)

		}
	}
	lessCost, smallIndex := smallestNonZeroIndex(cheapestCostArray[:])
	lessDuration, _ := smallestNonZeroIndex(durationArray)
	lessDistance, _ := smallestNonZeroIndexFloat(distanceArray)

	isVisited[smallIndex] = true
	visited[smallIndex] = true
	totalcost += lessCost
	totalduration += lessDuration
	totaldistance += lessDistance
	fmt.Println("TotalCost till now : ", totalcost)
	fmt.Println("TotalDuration till now : ", totalduration)
	fmt.Println("TotalDistance till now : ", totaldistance)
	fmt.Println("First closest destination :", t.Location_ids[smallIndex])
	fmt.Println("Visited destinations :", visited[smallIndex])

	if smallIndex > len(input)-1 {
		startingPointOID = bson.ObjectIdHex(input[len(input)-1])

	} else {
		startingPointOID = bson.ObjectIdHex(input[smallIndex])
	}

	if smallIndex > len(input)-1 {
		fmt.Println("HERE$$$$$")
		output = append(output, input[len(input)-1])
		input = append(input[:len(input)-1], input[len(input):]...)
	} else {
		output = append(output, input[smallIndex])
		input = append(input[:smallIndex], input[smallIndex+1:]...)
	}

	cheapestCostArray = cheapestCostArray[0:0]
	durationArray = durationArray[0:0]
	distanceArray = distanceArray[0:0]
	if len(input) > 0 {
		determineStartAndEndPoints(startingPointOID, input, output, rw)
	} else {
		if !startingPointVisited {
			startingPointVisited = true

			startingPointOID = bson.ObjectIdHex(output[len(output)-1])
			input = append(input, t.Starting_from_location_id)
			determineStartAndEndPoints(startingPointOID, input, output, rw)
		} else {

			status := "planning"
			output = output[:len(output)-1]

			c := mgoSession.DB("cmpe-273-sagardafle").C("ubertrip_details")

			oid := bson.NewObjectId()
			// Insert Datas
			err := c.Insert(&ResponseforPost{ID: oid, Status: status, Starting_from_location_id: t.Starting_from_location_id, Best_route_location_ids: output, Total_uber_costs: totalcost, Total_uber_duration: totalduration, Total_distance: totaldistance})

			if err != nil {
				panic(err)
			}

			m := &ResponseforPost{
				ID:     oid,
				Status: status,
				Starting_from_location_id: t.Starting_from_location_id,
				Best_route_location_ids:   output,
				Total_uber_costs:          totalcost,
				Total_uber_duration:       totalduration,
				Total_distance:            totaldistance,
			}
			js, err := json.Marshal(m)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}

			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(201)
			rw.Write(js)
		}

	}
}

func calculateEstimatesBetweenNodes(priceestimatestruct UberAPI, index int) (lowEstimate []int, duration []int, distance []float64) {
	fmt.Println("length of low estimate array is:", len(priceestimatestruct.Prices))
	lowEstimate = make([]int, len(priceestimatestruct.Prices))
	for i := 0; i < len(priceestimatestruct.Prices); i++ {
		lowEstimate[i] = priceestimatestruct.Prices[i].LowEstimate
	}

	duration = make([]int, len(priceestimatestruct.Prices))
	for i := 0; i < len(priceestimatestruct.Prices); i++ {
		duration[i] = priceestimatestruct.Prices[i].Duration
	}

	distance = make([]float64, len(priceestimatestruct.Prices))
	for i := 0; i < len(priceestimatestruct.Prices); i++ {
		distance[i] = priceestimatestruct.Prices[i].Distance
	}

	sort.Ints(lowEstimate)
	sort.Ints(duration)
	sort.Float64s(distance)
	fmt.Println("Unsorted LowEstimate array : ", lowEstimate)
	fmt.Println("Unsorted duration array : ", duration)
	fmt.Println("Unsorted distance array : ", distance)

	cheapestCostArray = append(cheapestCostArray, lowEstimate[0])
	durationArray = append(durationArray, duration[0])
	distanceArray = append(distanceArray, distance[0])
	return cheapestCostArray, durationArray, distanceArray
}

func smallestNonZeroIndex(s []int) (n int, i int) {

	//i = -1
	for _, v := range s {
		if v > 0 && (v < n || n == 0) {
			i = 0
			n = v
			i++
		}
	}
	return n, i
}

func smallestNonZeroIndexFloat(s []float64) (n float64, i int) {
	i = 0
	for _, v := range s {
		if v > 0 && (v < n || n == 0) {
			n = v
			i++
		}
	}
	return n, i
}

func getPriceEstimateAPIresults(priceestimatestring string) UberAPI {

	response, err := http.Get(priceestimatestring)
	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		err = json.Unmarshal(contents, &priceestimatestruct) // here!
		if err != nil {
			panic(err)
		}
	}
	return priceestimatestruct
}

func generatePriceEstimateURL(priceestimateapi string, start_latitude float64, start_longitude float64, end_Latitude float64, end_Longitude float64) string {
	start_latitude_string := strings.Replace(priceestimateapi, "<start_latitude>", strconv.FormatFloat(start_latitude, 'f', 6, 64), -1)
	start_longitude_string := strings.Replace(start_latitude_string, "<start_longitude>", strconv.FormatFloat(start_longitude, 'f', 6, 64), -1)
	end_latitude_string := strings.Replace(start_longitude_string, "<end_latitude>", strconv.FormatFloat(end_Latitude, 'f', 6, 64), -1)
	final_String := strings.Replace(end_latitude_string, "<end_longitude>", strconv.FormatFloat(end_Longitude, 'f', 6, 64), -1)
	return final_String
}

func getcoordinatesfromdatabase(oid bson.ObjectId) (float64, float64) {
	if err := mgoSession.DB("cmpe-273-sagardafle").C("user_details").FindId(oid).One(&m); err != nil {
		panic(err)
	}

	_, err := json.Marshal(m)
	if err != nil {
		//http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	startlatitude := m.Coordinate.Latitude
	startlongitude := m.Coordinate.Longitude
	fmt.Println("Destination Latitude", startlatitude)
	fmt.Println("Destination Longitude", startlongitude)
	return startlatitude, startlongitude
}

func updateTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	var tripputstruct TripPutOutput
	id := p[0].Value
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}
	oid := bson.ObjectIdHex(id)
	clonemgo()
	if err := mgoSession.DB("cmpe-273-sagardafle").C("ubertrip_details").FindId(oid).One(&tripputstruct); err != nil {
		w.WriteHeader(404)
		return
	}
	var temporary Interim_data
	var theStruct PutStruct
	var final PutStructForResponse
	final.finalMap = make(map[string]PutStruct)

	theStruct.trip_route = tripputstruct.Best_route_location_ids
	//adding starting location to the array of routes.
	theStruct.trip_route = append([]string{tripputstruct.Starting_from_location_id}, theStruct.trip_route...)
	fmt.Println("The route array is: ", theStruct.trip_route)
	theStruct.node_visited_map = make(map[string]int)

	var node_visited_list []string
	var node_unvisited_list []string

	if err := mgoSession.DB("cmpe-273-sagardafle").C("Trip_interim_data").FindId(id).One(&temporary); err != nil {
		for index, loc := range theStruct.trip_route {
			if index == 0 {

				theStruct.node_visited_map[loc] = 1
				node_visited_list = append(node_visited_list, loc)
			} else {
				theStruct.node_visited_map[loc] = 0
				node_unvisited_list = append(node_unvisited_list, loc)
			}
		}
		temporary.Id = id
		temporary.Trip_visited = node_visited_list
		temporary.Trip_not_visited = node_unvisited_list
		temporary.Trip_completed = 0
		mgoSession.DB("cmpe-273-sagardafle").C("Trip_interim_data").Insert(temporary)

	} else {
		for _, loc_id := range temporary.Trip_visited {
			fmt.Println("In else+ condition")
			theStruct.node_visited_map[loc_id] = 1
		}
		for _, loc_id := range temporary.Trip_not_visited {
			theStruct.node_visited_map[loc_id] = 0
		}
	}

	fmt.Println("node_visited_map :::: ", theStruct.node_visited_map)
	final.finalMap[id] = theStruct

	last_index := len(theStruct.trip_route) - 1
	trip_completed := temporary.Trip_completed

	if trip_completed == 1 {

		tripputstruct.Status = "completed"

		uj, _ := json.Marshal(tripputstruct)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		fmt.Fprintf(w, "%s", uj)
		return
	}

	for i, location := range theStruct.trip_route {
		if theStruct.node_visited_map[location] == 0 {
			tripputstruct.Next_destination_location_id = location
			nextoid := bson.ObjectIdHex(location)
			var o ResponseBody
			if err := mgoSession.DB("cmpe-273-sagardafle").C("user_details").FindId(nextoid).One(&o); err != nil {
				w.WriteHeader(404)
				return
			}
			endlat := strconv.FormatFloat(o.Coordinate.Latitude, 'g', -1, 64)
			endlong := strconv.FormatFloat(o.Coordinate.Longitude, 'g', -1, 64)

			if i == 0 {
				starting_point := theStruct.trip_route[last_index]
				startingoid := bson.ObjectIdHex(starting_point)
				var o ResponseBody
				if err := mgoSession.DB("cmpe-273-sagardafle").C("user_details").FindId(startingoid).One(&o); err != nil {
					w.WriteHeader(404)
					return
				}
				startlat := strconv.FormatFloat(o.Coordinate.Latitude, 'g', -1, 64)
				starlong := strconv.FormatFloat(o.Coordinate.Longitude, 'g', -1, 64)

				fmt.Println("startlat", startlat)
				fmt.Println("starlong", starlong)

				fmt.Println("endlat", endlat)
				fmt.Println("endlong", endlong)
				eta := getETA(startlat, starlong, endlat, endlong)
				tripputstruct.Uber_wait_time_eta = eta
				trip_completed = 1
			} else {
				other_start_point := theStruct.trip_route[i-1]
				other_start_point_string := bson.ObjectIdHex(other_start_point)
				var o ResponseBody
				if err := mgoSession.DB("cmpe-273-sagardafle").C("user_details").FindId(other_start_point_string).One(&o); err != nil {
					w.WriteHeader(404)
					return
				}
				startlat := strconv.FormatFloat(o.Coordinate.Latitude, 'g', -1, 64)
				starlong := strconv.FormatFloat(o.Coordinate.Longitude, 'g', -1, 64)
				fmt.Println("startlat", startlat)
				fmt.Println("starlong", starlong)

				eta := getETA(startlat, starlong, endlat, endlong)
				tripputstruct.Uber_wait_time_eta = eta
			}

			fmt.Println("Starting Location: ", tripputstruct.Starting_from_location_id)
			fmt.Println("Next destination: ", tripputstruct.Next_destination_location_id)
			theStruct.node_visited_map[location] = 1
			if i == last_index {
				theStruct.node_visited_map[theStruct.trip_route[0]] = 0
			}
			break
		}
	}

	node_visited_list = node_visited_list[:0]
	node_unvisited_list = node_unvisited_list[:0]
	for location, visit := range theStruct.node_visited_map {
		if visit == 1 {
			node_visited_list = append(node_visited_list, location)
		} else {
			node_unvisited_list = append(node_unvisited_list, location)
		}
	}

	temporary.Id = id
	temporary.Trip_visited = node_visited_list
	temporary.Trip_not_visited = node_unvisited_list

	temporary.Trip_completed = trip_completed

	c := mgoSession.DB("cmpe-273-sagardafle").C("Trip_interim_data")
	other_id := bson.M{"_id": id}
	err := c.Update(other_id, temporary)
	if err != nil {
		panic(err)
	}

	uj, _ := json.Marshal(tripputstruct)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)

}

func getETA(startingLatitude, startingLongitude, endingLatatitude, endingLongitude string) int {

	reqURL := "https://sandbox-api.uber.com/v1/requests"
	var etaString = []byte(`{"start_latitude":"` + startingLatitude + `","start_longitude":"` + startingLongitude + `","end_latitude":"` + endingLatatitude + `","end_longitude":"` + endingLongitude + `","product_id":"04a497f5-380d-47f2-bf1b-ad4cfdcb51f2"}`)
	request, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(etaString))
	request.Header.Set("Authorization", "Bearer <provided in email>")
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}

	var reseta UberETAEstimateStruct
	err = json.Unmarshal(body, &reseta)
	if err != nil {
		fmt.Println(err)
	}
	eta := reseta.ETA
	fmt.Println("ETA", eta)
	return eta

}

func clonemgo() {
	session, err := mgo.Dial("mongodb://sagardafle:sagardafle123@ds045454.mongolab.com:45454/cmpe-273-sagardafle")
	mgoSession = session
	if err != nil {
		panic(err)
	}

}

func planTrip(rw http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	output := make([]string, len(input))
	if err := json.NewDecoder(req.Body).Decode(&t); err != nil {
	}

	initialstartingPointOID = bson.ObjectIdHex(t.Starting_from_location_id)
	clonemgo()
	input := t.Location_ids
	determineStartAndEndPoints(initialstartingPointOID, input, output, rw)
}

func getTripDetails(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	trip_id := ps.ByName("trip_id")
	fmt.Println("ID from get", trip_id)
	clonemgo()
	oid := bson.ObjectIdHex(trip_id)
	fmt.Println("OID from get", oid)
	if err := mgoSession.DB("cmpe-273-sagardafle").C("ubertrip_details").FindId(oid).One(&rfp); err != nil {
		panic(err)
	}

	js, err := json.Marshal(rfp)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)

}

func main() {
	router := httprouter.New()
	router.POST("/trips", planTrip)
	router.GET("/trips/:trip_id", getTripDetails)
	router.PUT("/trips/:trip_id/request", updateTrip)
	log.Fatal(http.ListenAndServe(":8080", router))
}
