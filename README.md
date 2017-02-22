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

### Message Handle Process
Communication--->MessageClassifier--->Channel--->Consumerpool--->Target Communication

### File Structure
>  ***Channel.go***  
Implement channel which allows user-defined callbacks to handle messages and implement channel groups to 
uniformly dipatch message to channels and forward handled messages to consumer pool.
>  
***Communication.go***  
Use sse to perform persist connection between server and client. It uses SSEBroker and you should implement 
onConnection callback to set user-id.
>  
***ConsumerPool.go***  
ConsumerPool is used to dispatch messages efficiently. It uses the efficient 'worker pool' design to reuse message dispatching go routines. By reducing time over-head of creating a new go routine, it enables efficient message dispatch.
>  
***FileProxy.go***  
To trasfer files between clients, we use fileproxy to temporally create a url identifying a file resource so that web browser can automatically present a picture or show the url of a temporal file in serser for downloading.
>  
***IM.go***  
IM is the wrapper of WEB-IM, you can use it to create your web instance message application.
>  
***Message.go***  
Message struct defines a specific type of message. There are some pre-defined message types in it.
>  
***MessageClassifier.go***  
Classify messages and dispatch them to different channel gourps.
>  
***Protocal.go***  
Define communcation protocals 
>  
***SSEBroker.go***  
Define the sse broker to warp sse methods
>  
***UserManager.go***  
Define the action of managing friends or register a new user.
