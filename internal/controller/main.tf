variable "worker" {
type = object({
    name= string
    count = number
    availability_zone = string
})
 
}