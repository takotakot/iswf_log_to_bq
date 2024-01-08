data "google_project" "project" {}

resource "google_bigquery_dataset" "logs" {
  dataset_id = var.dataset_id

  location              = "US"
  storage_billing_model = "PHYSICAL"
}

resource "google_bigquery_table" "logs" {
  dataset_id = google_bigquery_dataset.logs.dataset_id
  table_id   = var.logs_table_id

  require_partition_filter = true

  time_partitioning {
    field = "request_time"
    type  = "DAY"
  }

  clustering = ["group_name", "account_name", "content_type", "determination_category"]

  schema = jsonencode(yamldecode(file("${path.module}/../../../common/schemata/logs.yml")))
}

# resource "google_bigquery_table" "load_template" {
#   dataset_id          = google_bigquery_dataset.logs.dataset_id
#   table_id            = var.load_template_table_id

#   schema = jsonencode(yamldecode(file("${path.module}/../../../common/schemata/load_template.yml")))
# }
