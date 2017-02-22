# Web-IM
A Web Instant Messaging Framework Built With Golang


### Design
- Use balanced load techniques when designing instant messaging service. Take full
advantage of multi-processor computing resources by using message classifier, processing
channel and message dispatcher. Use IoC technique to enable users to define their own
message process methods.
- Used the efficient ‘worker pool’ design to reuse message dispatching go routines. By
reducing time over-head of creating a new go routine, it enables efficient message
dispatch.
