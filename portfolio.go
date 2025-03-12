package main

import "net/http"


func PortfolioHandler(w http.ResponseWriter, r *http.Request){
	w.Write([]byte("hello"))
}