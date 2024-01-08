data "google_project" "project" {}

resource "google_project_service" "functions" {
  service            = "cloudfunctions.googleapis.com"
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
  account_id   = "load2logs-function-sa"
  display_name = "Load2logs Fuction Service Account"
}

# Grant permission to invoke Cloud Run services
resource "google_project_iam_member" "runinvoker" {
  project = data.google_project.project.id
  role    = "roles/run.invoker"
  member  = "serviceAccount:${google_service_account.default.email}"
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
  bucket = var.csv_bucket
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
    ]
  }

  name        = "load2logs"
  description = "load2logs file content from csv_bucket"
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
    trigger_region        = "us-central1"
    event_type            = "google.cloud.pubsub.topic.v1.messagePublished"
    pubsub_topic          = var.source_topic_id
    retry_policy          = "RETRY_POLICY_RETRY"
    service_account_email = google_service_account.default.email
  }
}
