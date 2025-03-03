
variable "region" {
  description = "AWS region"
  default     = "ap-south-1"
  type        = string
}

# Optional: Add cluster name variable for better reusability
variable "cluster_name" {
  description = "Name of the EKS cluster"
  default     = "scoutflo-assignment-cluster"
  type        = string
}

# Optional: Add node group configuration variables
variable "node_group_desired_size" {
  description = "Desired number of worker nodes"
  default     = 2
  type        = number
}

variable "node_group_instance_type" {
  description = "EC2 instance type for worker nodes"
  default     = "t3.medium"
  type        = string
}
