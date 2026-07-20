output "instance_id" {
  value = aws_instance.main.id
}

output "vpc_id" {
  value = aws_vpc.main.id
}
