output "dataset" {
  value = google_bigquery_dataset.logs
}

output "logs_table" {
  value = google_bigquery_table.logs
}

output "load_template_table" {
  value = ""
}
