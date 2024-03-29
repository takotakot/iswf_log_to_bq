data "google_project" "project" {}

resource "google_project_service" "functions" {
  service            = "cloudfunctions.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "run" {
  service            = "run.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "cloudbuild" {
  service            = "cloudbuild.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "eventarc" {
  service            = "eventarc.googleapis.com"
  disable_on_destroy = false
}

resource "google_service_account" "default" {
  account_id   = "bucket2logs-function-sa"
  display_name = "Bucket2logs Fuction Service Account"
}

resource "google_storage_bucket" "log_bucket" {
  name                        = var.log_bucket
  location                    = "US"
  uniform_bucket_level_access = true
}

data "google_storage_project_service_account" "gcs_account" {
}

# To use GCS CloudEvent triggers, the GCS service account requires the Pub/Sub Publisher(roles/pubsub.publisher) IAM role in the specified project.
# (See https://cloud.google.com/eventarc/docs/run/quickstart-storage#before-you-begin)
resource "google_project_iam_member" "gcs-pubsub-publishing" {
  project = data.google_project.project.id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${data.google_storage_project_service_account.gcs_account.email_address}"
}

# Grant permission to invoke Cloud Run services
resource "google_project_iam_member" "runinvoker" {
  depends_on = [google_project_iam_member.gcs-pubsub-publishing]
  project    = data.google_project.project.id
  role       = "roles/run.invoker"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_project_iam_member" "event-receiving" {
  depends_on = [google_project_iam_member.runinvoker]
  project    = data.google_project.project.id
  role       = "roles/eventarc.eventReceiver"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_project_iam_member" "artifactregistry-reader" {
  depends_on = [google_project_iam_member.event-receiving]
  project    = data.google_project.project.id
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_storage_bucket_iam_member" "object-input" {
  bucket = google_storage_bucket.log_bucket.name
  role   = "roles/storage.objectUser"
  member = "serviceAccount:${google_service_account.default.email}"
}

resource "google_project_iam_member" "default" {
  project = data.google_project.project.id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.default.email}"
}

resource "google_bigquery_dataset_iam_member" "logs" {
  dataset_id = var.dataset_id
  role       = "roles/bigquery.dataEditor"
  member     = "serviceAccount:${google_service_account.default.email}"
}

resource "google_cloudfunctions2_function" "default" {
  depends_on = [
    google_project_service.functions,
    google_project_service.run,
    google_project_service.cloudbuild,
    google_project_service.eventarc,
    google_project_iam_member.event-receiving,
    google_project_iam_member.artifactregistry-reader,
  ]
  lifecycle {
    ignore_changes = [
      service_config[0].service,
      service_config[0].service_account_email,
      build_config[0].entry_point,
      build_config[0].docker_repository,
    ]
  }

  name        = "bucket2logs"
  description = "Bucket2logs file content from log_bucket"
  location    = "us-central1"

  build_config {
    entry_point = "template"
    runtime     = "go121"
    source {
      storage_source {
        bucket = var.source_archive_bucket
        object = var.source_archive_object
      }
    }
  }
  service_config {
    available_memory = "128Mi"
    environment_variables = {
      PROJECT_ID = data.google_project.project.project_id
      DATASET_ID = var.dataset_id
      TABLE_ID   = var.logs_table_id
    }
    ingress_settings                 = "ALLOW_ALL"
    max_instance_count               = 1
    max_instance_request_concurrency = 1
    min_instance_count               = 0
    timeout_seconds                  = 60
    service_account_email            = google_service_account.default.email
  }
  event_trigger {
    trigger_region        = "us"
    event_type            = "google.cloud.storage.object.v1.finalized"
    retry_policy          = "RETRY_POLICY_RETRY"
    service_account_email = google_service_account.default.email
    event_filters {
      attribute = "bucket"
      value     = google_storage_bucket.log_bucket.name
    }
  }
}
