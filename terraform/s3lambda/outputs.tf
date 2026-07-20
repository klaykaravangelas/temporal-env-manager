output "bucket_name" {
  value = aws_s3_bucket.main.bucket
}

output "lambda_function_name" {
  value = aws_lambda_function.s3_handler.function_name
}

output "lambda_arn" {
  value = aws_lambda_function.s3_handler.arn
}
