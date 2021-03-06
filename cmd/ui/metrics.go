package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	booksProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "booksing_books_processed",
		Help: "The number of processed books",
	}, []string{"transaction"})
	booksProcessedTime = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "booksing_books_duration_seconds",
		Help: "The time taken to process the books in seconds",
	}, []string{"transaction"})
	searchErrorsMetric = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "booksing_search_errors",
		Help: "The number of errors encountered when contacting search",
	}, []string{"type"})
	dbErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "booksing_db_errors",
		Help: "The number of errors encountered when using the db",
	}, []string{"type"})
	statusGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "booksing_indexing",
		Help: "Wether booksing is indexing or not",
	})
	totalBooksGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "booksing_books_in_index",
		Help: "Total number of books available for searching",
	})
)
