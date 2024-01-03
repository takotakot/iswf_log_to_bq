output "output_bucket" {
  value = google_storage_bucket.output_bucket
}

output "output_topic" {
  value = google_pubsub_topic.notify_topic
}
