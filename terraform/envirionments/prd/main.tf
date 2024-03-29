data "archive_file" "funciton-template-archive" {
  type        = "zip"
  source_dir  = local.funciton_template_dir
  output_path = "${local.artifact_dir}/funciton_template.zip"
}

module "source_archive_bucket" {
  source                = "../../modules/source_archive_bucket"
  source_archive_bucket = "${local.project_id}_artifact"
}

resource "google_storage_bucket_object" "funciton-template-gcs-archive" {
  source = data.archive_file.funciton-template-archive.output_path

  bucket = module.source_archive_bucket.source-archive-bucket.name
  name   = "funciton-template.zip"
}

module "unzip" {
  source                = "../../modules/unzip"
  zip_bucket            = "${local.project_id}_zip"
  output_bucket         = "${local.project_id}_tgz"
  notify_topic          = local.unzip_notify_topic
  source_archive_bucket = google_storage_bucket_object.funciton-template-gcs-archive.bucket
  source_archive_object = google_storage_bucket_object.funciton-template-gcs-archive.name
}

module "untar" {
  source                = "../../modules/untar"
  tar_bucket            = "${local.project_id}_tgz"
  output_bucket         = "${local.project_id}_csv"
  source_topic_id       = module.unzip.output_topic.id
  notify_topic          = local.untar_notify_topic
  source_archive_bucket = google_storage_bucket_object.funciton-template-gcs-archive.bucket
  source_archive_object = google_storage_bucket_object.funciton-template-gcs-archive.name
}

module "bigquery" {
  source        = "../../modules/bigquery"
  dataset_id    = "logs"
  logs_table_id = "logs"
}

module "load2logs" {
  source                = "../../modules/load2logs"
  csv_bucket            = "${local.project_id}_csv"
  source_topic_id       = module.untar.output_topic.id
  dataset_id            = "logs"
  logs_table_id         = "logs"
  source_archive_bucket = google_storage_bucket_object.funciton-template-gcs-archive.bucket
  source_archive_object = google_storage_bucket_object.funciton-template-gcs-archive.name
}

module "bucket2logs" {
  source                = "../../modules/bucket2logs"
  log_bucket            = "${local.project_id}_log"
  dataset_id            = "logs"
  logs_table_id         = "logs"
  source_archive_bucket = google_storage_bucket_object.funciton-template-gcs-archive.bucket
  source_archive_object = google_storage_bucket_object.funciton-template-gcs-archive.name
}
