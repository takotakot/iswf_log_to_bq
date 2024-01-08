locals {
  project_id            = local.secret_project_id
  region                = "asia-northeast1"
  funciton_template_dir = "${path.module}/../../../function_template"
  artifact_dir          = "../../../artifacts"
  unzip_notify_topic    = "unzip"
  untar_notify_topic    = "untar"
}
