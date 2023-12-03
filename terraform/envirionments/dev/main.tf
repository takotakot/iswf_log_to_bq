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
