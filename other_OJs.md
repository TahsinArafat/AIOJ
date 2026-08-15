

## Online Judge System: Requirements, Architecture,
and Experiences
## Yutaka Watanobe
## *
## ,
## ‡
## , Md. Mosta ̄zer Rahman
## *
## ,
## †
## ,
## §
## , Taku Matsumoto
## *
## ,
## ¶
## ,
## Uday Kiran Rage
## *
## ,
## ||
and Penugonda Ravikumar
## *
## ,
## **
## *
Department of Computer Science and Engineering
The University of Aizu, Aizuwakamatsu, 965-8580, Japan
## †
Department of Computer Science and Engineering
Dhaka University of Engineering & Technology, Gazipur, 1707, Bangladesh
## ‡
yutaka@u-aizu.ac.jp
## §
mostafiz26@gmail.com
## ¶
t.matsumoto556@gmail.com
## ||
udayrage@u-aizu.ac.jp
## **
raviua138@gmail.com
## Received 9 April 2022
## Revised 19 May 2022
## Accepted 20 May 2022
## Published 4 July 2022
The development and operation of Online Judge System (OJS), which is used to evaluate the
correctness of programs, is a nontrivial and di±cult task due to the various functional and non-
functional requirements. However, although many OJSs have been developed and operated, and
their usefulness reported, the theory for constructing OJSs has not been su±ciently discussed. In
this paper, we present the functional and nonfunctional requirements oriented to OJS as well as
demonstrate the internal components and software architecture of an OJS, which has been in
operation for over a decade and has evaluated over six million solutions. We also present real-
world experiences and challenges encountered during this long journey of our OJS.
Keywords: Online judge system; online judge system architecture; online judge system
requirements; programming support systems.
## 1. Introduction
Online Judge Systems (OJSs) currently provide important services for people
involved in programming [1]. OJSs are playing a key role in both academia and
industry to evaluate the correctness of programs submitted by the learners. Since the
## ‡,§
Corresponding authors.
This is an Open Access article published by World Scienti ̄c Publishing Company. It is distributed under
the terms of the Creative Commons Attribution 4.0 (CC BY) License which permits use, distribution and
reproduction in any medium, provided the original work is properly cited.
## OPEN ACCESS
International Journal of Software Engineering
and Knowledge Engineering
## Vol. 32, No. 4 (2022) 1–30
## #
## .
c
## The Author(s)
## DOI:10.1142/S0218194022500346
## 1
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

advent of OJS, its educational bene ̄ts and availability for competition, training, and
personnel evaluation have been demonstrated. However, the requirement of
executing, assessing, and managing solution codes submitted online by arbitrary
users is a challenging problem, which has been explored for a long time.
To provide various functionalities, the components of an OJS typically comprise
of web clients, a repository of problem sets (questions), and multiple servers. Among
the components, the core element of an OJS is the judge system, within which load
balancing, evaluation, and noti ̄cation processes must be performed in a rigorous and
e±cient manner. To be precise, an OJS accepts solution codes from arbitrary users,
compiles and executes them in a shared environment, validates the behavior of them
using speci ̄c test input/output datasets, and then reports the results of the evalu-
ation and resource usage to all associated users. While meeting these functional
requirements, the OJS has to ful ̄ll various non-functional requirements depending
on the use case.
Most of the relevant studies in this area were primarily concerned with improving
the functionalities of OJS [2,3] as well as with security issues [4]. Instead of the global
deployment of OJS, most cases were developed and implemented for speci ̄c pur-
poses including programming contests, academic course management, and corporate
training. Other trends of the development of OJS include visualization techniques for
statistical reports; technology adaptations such as containers (docker) with micro-
services; and data science for learning support [5
## –
10]. However, although we found
many studies proposing distinctive OJSs and their extensions, the complete internal
architecture and implementation have rarely been discussed, clari ̄ed, or evaluated
through the long-term operation. This indicates that the construction theory nec-
essary for the further development of OJS has also not been su±ciently discussed. In
contrast, the diversity and availability of OJSs have led to their widespread use and
rapid increase in recent years, and we need to expand and further develop the non-
functional requirements of OJSs.
To alleviate this research gap, in this paper, we reveal the internal architecture
and implementation of our original OJS, the Aizu Online Judge (AOJ) [11]. AOJ
provides the most common services of regular OJS for over a decade. AOJ has over
100,000 registered users and has consistently assessed over six million program codes.
AOJ has recognized as one of major OJSs which provides APIs for accessing its
services and data as well as resource archives. As a result, the data generated by AOJ
have been included in the Project CodeNet by IBM, a dataset oriented to AI for Code
generated by AOJ [12]. This paper is a substantially extended version of our con-
ference paper [29], which presented limited contents and experiments. Herein,  ̄rst
the requirements of OJS are presented using data elements provided/generated by
the users/system. Non-functional requirements oriented to OJSs are also discussed
with the possible trade-o®. Subsequently, we present the internal architecture of
AOJ focusing on its main components of the load balancer, broadcaster, and judge
system. We also present the experiences and challenges encountered during the 10
years of operation. The results of the data analysis showed that our system, which
2Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

uses the presented architecture, was running stably while satisfying several key non-
functional requirements. In other words, this paper makes the following contribu-
tions:
.Identifying the functional and non-functional requirements of OJSs
.Presenting the components and software architecture of the OJS as a theoretical
basis to build an OJS
.Demonstrating the validity of the theory by examining the state and performance
of the OJS, which is successfully running over a 10-year period
The rest of this paper is organized as follows. Section2discusses related work,
while Sec.3discusses OJS requirements. In Secs.4and5, the OJS component and
software architectures are presented, respectively. Section6discusses our experiences
and challenges, and we conclude this paper in Sec.7.
## 2. Related Work
Although the main purpose of this paper is to present the theoretical basis to build an
OJS, in this section, we discuss the most prominent OJSs presented in other studies.
First, we should mention traditional OJSs, such as UVa Online Judge (UVa OJ) [13]
and Sphere Online Judge (SPOJ) [14] among others like Timus Online Judge and
Peking University Online Judge (POJ). UVa OJ is one of the  ̄rst prominent judge
systems to receive submissions worldwide. The submissions were analyzed based on
10 years of experience with millions of submissions [15]. In addition, various models
including communication and data from the EduJudge project are presented [16]. In
this project, programming problems are de ̄ned as learning objects that are com-
patible with learning management systems and automated assessment platforms.
SPOJ is another innovative OJS that successfully provides regular judging ser-
vices (i.e. programming competitions and programming practices). The implemen-
tation of the SPOJ system with security measures and their operational experiences
are presented in [17]. Moreover, there are many programming contest platforms (e.g.
Codeforces, AtCoder, and TopCoder) that regularly host programming contests with
powerful and scalable judging systems. Furthermore, many companies (e.g. Hack-
erRank and LeetCode) also operate typical OJS for providing specialized assistance,
such as programmer recruitment, job search, programming practice and hiring.
Software that can be implemented in a local machine and provide functions for
managing original questions and assessment methods are not only appealing for
holding competitions but also for organizing an OJS. CMS [18] and DOMjudge [19]
are prominent examples of active open-source projects for contest management that
can also be used in education. Although these systems can provide a secure and
reliable environment and allow for °exible management, the closed systems are
designed to meet only a few non-functional requirements as an OJS system.
The trend in recent years is the technology to easily deploy an OJS. Yiboet al.[20]
proposed a small-scale and exercise course-oriented OJS that relies on the Docker
Online Judge System: Requirements, Architecture, and Experiences3
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

Container technique to reduce the maintenance cost of traditional OJSs. It also reduces
reliance on large hardware resources, and mitigate the damage caused by major de-
velopment cycles. This course-oriented OJS is easier to manage compared to tradi-
tional OJSs. In addition to experimental results, various internal system structures,
modules, implementation mechanisms and evaluation procedures were presented.
There are also literature works discussing architecture. Lianget al.[21] proposed a
system for evaluating program code called Online Automated Judging (OAJ). They
described the structural design of the OAJ system. The OAJ system consists of two
parts: sandbox judging and user interface (web). The OAJ system incorporates
several improved techniques, such as thread pools for parallel testing and a hori-
zontal split table for better resource utilization and system performance. Further-
more, an overview of the OAJ system's architecture, hierarchical structure, judging
process, and input and output streams splitting has been presented. Meanwhile, a
MetaOJ [22] is designed to be a distributed, scalable and fault-tolerant OJS by
adding several interfaces and instances to the conventional OJS. MetaOJ introduces
several unique mechanisms for integrating cloud computing infrastructure and au-
tomating OJS instance deployments.
## Although,asmentionedearlier,therehasbeenasigni ̄cantincreaseinresearchonthe
development of various services using OJS, the important and major components of an
OJS, collectively referred to as online code execution systems (OCESs), are hardly
discussed in the literature. Based on the literature review, we also found that OCES was
̄rstdevelopedbytheSphereEngine[23]asacommercialandclosed-sourceproduct.The
Camisole [24] system is another type of OCES that applies sandboxing approach. There
is a wide variety of sandboxing techniques to handle untrusted solution codes. Basically,
these techniques are implemented to allocate custom resources based on containers or
virtual machines. Normally, a common technique called sandbox isolation is used in
regular judge systems to safely compile and execute solution codes.
The internal architectures of OJS have recently begun to be discussed, rather than
being actively presented. As an example, the architecture for a robust and scalable code
execution system (CES) is presented [25]. An advanced CES is Judge0 [26], which uses
a sandbox isolation technique. Judge0 provided an open-source application program
interface for OCES that can be integrated with both web and OJS platforms. Mean-
while, a comprehensive open-source Automated Programming Assessment System
(APAS) named Edgar has been developed [27]. The complete architecture of the Edgar
is being used to monitor, analyze, and visualize the students' data as well as the quality
of the questions of the concerned subject. Edgar has been in use for the past 3 years and
it runs on all major operating systems. Edgar is built in a modular format that allows it
to be customized and scaled to  ̄t any need. SIETTE [28] is another type of APASs for
complex programming tasks developed based on two renowned assessment theories
such as classical test theory (CTT) and item response theory (IRT).
As mentioned above, since the  ̄rst OJS was devised, its educational bene ̄ts and
availability for competition, training, and personnel evaluation have been demon-
strated. In general, building an OJS independently is very costly, requiring hardware,
4Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

software, maintenance, and ongoing support. Therefore, recently, research on
mechanisms that allow third parties to easily deploy OJSs has become mainstream.
Consequently, attention has been focused on non-functional requirements such as
security, portability, and maintainability. These are the results of applying recent
technological advances such as virtualization, containerization, and cloud comput-
ing. Essentially, OJS, which is built on the cloud and is always online and equally
available to everyone, should focus on non-functional requirements such as imme-
diacy, consistency, robustness, sustainability, transparency, and availability.
- Requirements of the Online Judge System
In this section, the functional requirements of an OJS are presented through data
elements involved in the core components. Non-functional requirements are also
discussed which di®er depending on the application policies.
3.1.Functional requirements
An OJS makes it possible to conduct learning, testing, comparing, and bench-
marking within a common environment with learning materials. In general, the OJS
receives the online submissions of solution codes aimed at speci ̄c tasks from learners,
then the system evaluates their correctness and performance. From the user's point
of view, the solution code is run and assessed on the cloud, not on the user's local
environment.
In a typical OJS, many problem sets are provided and many users are registered.
The OJS receives a submission requestQ¼fu;p;source;lang;J
u
;typegwith a
program codesourcewritten in the programming languagelangfrom a useruwhich
is trying to solve a problemp. When thetype2fCUSTOM;ACTUALgisCUSTOM,
the submission requestQcontains a user-de ̄ned judge dataJ
u
## ¼finput
u
## ;output
u
g
for  the  judge.  Conversely,  when  thetypeisACTUAL,  the  judge  data
## J
p
¼fR;I;O;G;V;ncases;eps;checker;reactivegprepared for a problempis used
for the judge as mentioned below. Judge dataJ
p
is used to evaluate the program
oriented to problemp.J
p
includes a set of assumed solutionsR, judge inputs and
outputsI¼fin
## 1
## ;in
## 2
## ;...;in
ncases
g,O¼fout
## 1
## ;out
## 2
## ;...;out
ncases
g, data generator
## G¼fg
## 1
## ;g
## 2
## ;...;g
m
g, and data validatorV¼fv
## 1
## ;v
## 2
## ;...;v
k
g. The judge input/out-
put consists of a set ofncases2Ntest cases, as pairs of inputin
i
and the corre-
sponding correct outputout
i
(i¼1;2;...;ncases). For each problemp, a metrics
## M
p
¼fflag;timeLim;memLimgis de ̄ned to evaluate solutions to it. The evalua-
tion metricsM
p
, identi ̄cation numberj, and timestampts
receive
are added to the
submission requestQ, which becomes the submissionS
j
## ¼fj;ts
receive
## ;u;p;source;
lang;J
u
;type;M
p
gsent to the judge.
There are four di®erent ways identi ̄ed byflag2fDIFF;ERROR;SPECIAL;
REACTIVEgto assess a program code. When theflagisDIFF, the OJS judges the
correctness of the program code based on the text-based di®erences between the
Online Judge System: Requirements, Architecture, and Experiences5
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

standard output of thesourceand the output prepared by the judge. When theflag
isERROR, both the standard output of thesourceand the output prepared by the
judge comprises real numbers, and the OJS judges correctness by allowing errors
within the valueeps2Rset in problemp. When theflagisSPECIAL, the OJS
judges the correctness of the program code using a special veri ̄cation program
checkerprepared for problemp. Thecheckerreads the standard output ofsource,
the judge inputin
i
2I, and the judge outputout
i
2Oto make the decision. For
these assessment methods, thesourcemust be prepared in such a way that it reads
the input data from the standard input and writes the calculation results to the
standard output. Conversely, when theflagisREACTIVE, the OJS judges the
correctness by the reactive programreactivededicated to problempthat interacts
with the user programsourcewith the judge data.
OJS returns the result of the judgmentsVR2fAC;WA;PE;CE;RE;TLE;MLE;
OLEgas the evaluation value. OJS returns Compile Error (CE) with a messagemsg
if thesourcecannot be compiled correctly bylang. OJS returns Runtime Error (RE)
with a messagemsgif thesourcecauses a runtime error during its execution.REcan
be caused, for example, by stack over°ow, out-of-range array references, zero divi-
sion, or other unexpected termination. OJS, respectively, returns Time Limit
Exceeded (TLE) or Memory Limit Exceeded (MLE) if the Central Processing Unit
(CPU) usaget
i
exceedstimeLim2Nor memory usagem
i
exceedsmemLim2N
during its execution. In other words, OJS evaluates the performance ofsourcein
terms of CPU time and memory usage for each test case, and if each of these exceeds
the value speci ̄ed in problemp, the performance is judged as insu±cient. OJS
returns Output Limit Exceeded (OLE) if the size of the standard output of the
sourceexceeds the speci ̄ed limit. If theflagisDIFF, OJS returnsWA(Wrong
Answer) when the standard output ofsourceforin
i
does not matchout
i
. However, if
there is a di®erence only in the display format, it returns Presentation Error (PE).
WAis caused by a variety of factors, including logical errors and algorithmic mis-
takes that cannot be detected by the compiler. For judges where theflagis
SPECIALorREACTIVE, OJS returnsWAaccording to the judgment of the cor-
responding dedicated program. Otherwise, OJS returns Accepted (AC) based on the
fact that all the test cases have been passed without the above-mentioned failures.
Thus, the verdicts (VR) of the OJS can be expressed mathematically by the following
equations:
VR¼AC()ð8
i
## VR
i
¼ACÞ^ð8
i
t
i
timeLimÞ^ð8
i
m
i
memLimÞ;ð1Þ
## VR¼VR
j
## ()ð8
i<j
## VR
i
¼ACÞ^ðVR
j
6¼ACÞ:ð2Þ
CPU time and memory usage required for each test case are packed intoTL¼
ft
## 1
## ;t
## 2
## ;...;t
nsucc
gandML¼fm
## 1
## ;m
## 2
## ;...;m
nsucc
g;respectively, wherensuccis the
number of successful test cases. The performance of the source codesourceis de ̄ned
bycpu¼maxðt
i
## Þandmem¼maxðm
i
Þ. Then, the evaluation result is de ̄ned by
## A
j
¼fj;VR;cpu;mem;msg;nsucc;ncases;TL;ML;output;ts
finish
g.
6Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

3.2.Non-functional requirements
Non-functional requirements refer to all requirements in software design other than
its functional requirements. They provide quality attributes to the system, and in
general, there is a wide range of items to be de ̄ned. Herein, we discuss some key non-
functional requirements that are oriented to OJS. Figure1shows a list of non-
functional requirements for OJS and their possible trade-o®.
Immediacy.Because an OJS is always asked to generate feedback in real-time, it
is important to shorten the response time between the request and corresponding
assessment. This requirement is one of the metrics that most a®ect quality and user-
friendliness. However, each individual process and the overhead of communication
can a®ect the latency of other requests as resources are shared in the cloud.
Security.Because an OJS executes any solution code, security is one of the main
concerns. The submitted solution code should not a®ect core operations such as
internal  ̄le access or processes. Most importantly, a secure OJS must prevent ma-
licious solution codes from a®ecting systems of the external world. However, a high
security setting impacts performance. As a result, the secured architecture and ad-
ditional scrutiny may come at the expense of immediacy.
Consistency.The system assumes that the performance of a solution code for a
task is compared with other solution codes (even for the same user). Therefore, a
legitimate and reproducible evaluation must be performed in terms of performance
and consistency. Thus, the system may be deployed in a centralized dedicated server
cluster where the environment is shared by the users under the same computational
resources.
Fig. 1. Non-functional requirements for OJS.
Online Judge System: Requirements, Architecture, and Experiences7
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

Newness.Because the hardware of the OJS a®ects the execution performance, it
should be upgraded promptly especially when better CPU and memory are available.
It is also ideal to update the compilers and runtime environments to support the
latest versions of programming language. However, frequent updates may make
inconsistencies between old submission records and the latest ones.
Robustness.The system must always be available and running 24 h a day, even
when no administrator is present. The system must also be able to handle certain
solution codes that consume excessive CPU cycles, memory, and stack resources.
The remaining processes, such as memory leaks and zombies, also be monitored. The
system has to keep responding, even if it is not working well to provide the assess-
ment service.
Correctness.The system must not make unfair assessments. More importantly,
it must not judge a wrong solution as a correct solution nor judge a correct solution
as a wrong solution. The measurements made must be accurate and must not un-
derestimate or overestimate. For a system that requires trust, if it is not working well
to provide correct assessment, it is better to stop the service or keep silent. However,
such an assertion may sacri ̄ce robustness.
Sustainability.In general, OJS-related services store data about users,
submissions, verdicts and other logs. Because solutions are re-evaluated, compared,
and shared as knowledge with common metrics, the judge system must remain
continuously with the associated data.
Portability.The OJS can be ported to other hardware systems. The portable
system can be easily installed and deployed by third parties to provide services on
demand. However, the portability may sacri ̄ce the sustainability of data because of
thead hocdeployment during the speci ̄c period and location.
Transparency.Processing status in OJS should be widely publicized, and de-
tailed reports should be available as quickly as possible. In addition, the evaluation
results should be noti ̄ed not only to the user but also to all related online target
groups. Such reports should include the judgment results for the corresponding
submitted solution codes as well as the status change of submissions in the queue.
Scalability.The OJS should be able to scale up according to the number of users
and its load of requests. It may enhance the immediacy and robustness (by fault-
tolerant). However, the scale-out may make the system di±cult to manage sub-
mission requests and disclose their status.
Availability.The OJS must be continuously running and ready for submission
at any given time. In general, availability is expressed as a percentage of system
availability over a period of time. In other words, OJS is required to minimize the
time of inactivity as much as possible.
Economic.The OJS is composed of parallel machines to increase its performance
and robustness. Moreover, the machines may be idle most of the time. Therefore,
e®orts to increase the e±ciency of power usage, such as starting up on demand,
should be considered to satisfy both availability and economic.
8Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

## 4. Component Architecture
An OJS is implemented and deployed based on an architecture taking into account
both functional and emphasized non-functional requirements. A service by OJS
oriented to several non-functional requirements cannot be realized by a monolithic
architecture. Instead, multiple-dedicated components work together to provide
quality-maintained functionality. In this section, we present the functionality of the
complete OJS architecture of the AOJ focusing on the data associated with the OJS,
not on the feature of other services.
Figure2shows a component diagram that provides an overview of the architec-
ture in which the major components of the OJS communicate with data related to
assessment. In principle, it is based on the idea of Micro Service Architecture (MSA)
to integrate lightweight loosely coupled components more °exibly. In this paper,
details of Load Balancer, Broadcaster, and Judge Cluster which organize the core
part of OJS, are presented. We also brie°y explain Web Server, Database Server, and
Judge Master. Although the design of database schema, APIs, and associated user
interfaces are the basis for determining the use case, their details are outside the
scope of this paper.
4.1.Web server
Web Server consists of multiple Web servers to provide di®erent APIs through
di®erent servers (hardware). A Web server is an interface between the external
systems and the OJS. The external includes terminal machines of users, adminis-
trators and other audiences. So, di®erent business logics are implemented in the Web
servers including registration, authentication, browsing and submission. To com-
municate with the OJS, the Web servers provide data based on a resource-oriented
architecture without page generation when it receives a request via a public API. A
submission requestQis received through the Web server. The metricM
p
, which is
obtained from the database server by the problem identi ̄erp, is accompanied byQ.
ThenQandM
p
are sent to the load balancer and the assessment resultA
j
becomes
Fig. 2. Primary OJS components.
Online Judge System: Requirements, Architecture, and Experiences9
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

available after the judge. Administrators also manage data in the database servers
and the judge master through the Web servers.
4.2.Database server
Database Server also consists of multistage database servers which are connected to
the Web servers. The database servers interact with the Web servers and the load
balancer via private APIs. It stores all information related to the service such as the
problem set, evaluation results information, and registered users. All solution codes
and statistical data are also accumulated and managed. As mentioned above, these
data and logs are managed by multistage databases for providing di®erent types
of APIs.
4.3.Judge master
Judge Master is an ancillary server to manage the judge data for Judge Cluster. It is a
kind of database server that is dedicated to manage judge data. Because it also
contains the continuous integration mechanism, we consider it as one of the special
components for OJS from the management point of view. One of the notable features
of our OJS architecture is that the payload between a judge server and the load
balancer does not contain a judge dataJ
p
, which is relatively heavy. Instead, the
judge master maintains a set of judge data for all problems, and their clones
are deployed to all judge servers before the corresponding problem is enabled.
Furthermore, when the system administrator adds or updates an element in the
judge dataJ
p
, a continuous integration mechanism is activated on the judge master,
and it maintains the data resources for all judge servers consistently. In the con-
tinuous integration,  ̄rst of all, subsets ofI(in
i
) are generated by elements ofG(g
i
## ).
Next, all elements inI(in
i
) are validated by elements ofV(v
i
), each of which checks
the correctness ofin
i
from di®erent constraints perspectives. Then, one of the ref-
erence solutionsr
i
fromRis built and reference outputout
i
for each test casein
i
is
generated. Solutions of otherr
i
are also built and their output is generated for all the
test cases in the same manner. Then, after all solutions successfully generating cor-
rect output which matches the correspondingout
i
, respectively, the judge master
makes the re ̄ned judge data available for all judge servers. At the same time, a
necessarycheckerorreactiveprogram is built on all judge servers.
4.4.Load balancer
The load balancer is a core component of OJS to mediate the Web servers and the
judge servers. It also communicates with database servers and the broadcaster. The
role of the load balancer is to perform load balancing for a number of submissions.
The submissions should be maintained by safe guarded queues without any loss and
duplication. Internally, the load balancer is mainly responsible for scheduling all
10Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

submissions requestQ. First, a unique judge identi ̄erjis assigned to the submission
requestQaccompanied by its metricsM
p
. It transforms the submission requestQto
## S
j
and allocates the submission to the corresponding judge server via the private
API. Pending submissions are managed by queues oriented to the corresponding
judge server. The load balancer is also responsible for managingA
j
received from the
judge servers. It also sends status noti ̄cation ofS
j
in queues and correspondingA
j
to
all users through di®erent channels. The noti ̄cations regardingA
j
are triggered for
the Web server, database server, and broadcaster.
4.5.Broadcaster
Generally, waiting times for submissions are di®erent depending on the execution
time of the program codes in the queues. The fact may change the order of the
assessment while a number of users are waiting for the results. So, the aim of
the broadcaster is to asynchronously notify status change of submission managed by
the load balancer.
In fact, the noti ̄cation of status changes related toS
j
andA
j
is performed using
three di®erent channels. The  ̄rst channel is a public API used to provide persistent
status information from the storage managed by one of the database servers in
response to certain query parameters received from the Web servers. Similarly, the
second channel is also a public API that provides the status of all recently posted
submissions in the queues (memory) managed by the load balancer. The third
channel is intended for real-time noti ̄cation and is based on asynchronous com-
munication by the broadcaster. So, this component does not store any data, but it is
dedicated to provide quick noti ̄cation. The broadcaster not only reduces the stress
of users in a waiting state but also reduces the number of queries to the Web server,
which in turn reduces the load on the load balancer.
4.6.Judge cluster
The load balancer communicates with judge servers in Judge Cluster. A judge server
executes a series of tasks for a request. First, it receives a submissionS
j
, then runs it
based on the judgment dataJ
p
, and  ̄nally, it sends back the resultA
j
. The judge
cluster must be physically (or at least virtually) isolated so that only authorized
processes can connect to the judge server through a private API. It is important to
prevent data leakage due to malicious or unintended operations. To improve the
overall service quality of the judgment, the judge cluster is formed with several judge
servers, taking into account the following points:
.For consistency, each judge server uses dedicated hardware resources exclusively
to run a given solution code
.For immediacy, a number of given solution codes can be run in di®erent judge
servers simultaneously
Online Judge System: Requirements, Architecture, and Experiences11
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

.For robustness, judge servers support each other as the parallel machines
.For scalability, the server cluster can be scaled up easily
## 5. Software Architecture
In this section, we present the details and features of the OJS software architecture,
focusing on the load balancer, broadcaster, and judge server.
5.1.Load balancer
The load balancer receives submission requests from one of the Web servers and then
distributes these requests to the corresponding judge servers. Moreover, it reports the
evaluation results through di®erent channels. The class diagram of the load balancer
is shown in Fig.3. The components of the load balancer are organized by multi-
threaded processes that contain three di®erent instances of SubmissionReceiver,
SubmissionSender, and StatusProvider. An instance of SubmissionReceiver receives
the submissions from the Web server, an instance of SubmissionSender sends sub-
missions to the corresponding judge server, and an instance of StatusProvider
responds status of the submissions. Submissions and evaluations are managed by two
Fig. 3. Main load balancer classes.
12Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

di®erent types of queues shared by the threads. One type waits for the submissionS
j
## ,
and the other waits for assessment resultsA
j
## .
Once the load balancer is activated, the instance of SubmissionSender generates
Nthread objects of SenderConnection, each of which manages a waitingSubmission
queue of the associated judge server (whereNrepresents the number of available
judge servers). This implies that the load balancer manages totallyNwait-
ingSubmission queues. The instance of SubmissionReceiver generates a thread object
of ReceiverConnection after receiving a submission requestQfrom the Web server
and placesS
j
in the corresponding waitingSubmission queue. The object of Recei-
verConnection allocates the identi ̄erjand decides the judge server (the object of
SenderConnection) based on the criteria, which can be de ̄ned according toS
j
and
the status of the queues. Subsequently, each object of SenderConnection places the
submissions into its own queue one by one and manages the assessments. The in-
stance of StatusProvider collects the contents of the queues and makes them avail-
able to the Web server upon request. Further, the load balancer has two other
instances of StatusBroadcaster and VerdictUpdator to connect the broadcaster and
database servers, respectively.
The thread objects in the load balancer manage theS
j
andA
j
, as shown in Fig.4.
In this °ow, an instance of SenderConnection is added by scaling up the judge
cluster. After receiving a submissionS
j
, the instance of ReceiverConnection places it
in the corresponding waitingSubmissions queue. Then, the instance of SenderCon-
nection sends this pending submission at the front of its queue to the corresponding
judge server. The instance of SenderConnection waits for a response from the judge
server until the assessment resultA
j
is available. The completed submission is added
to the judgedSubmission queue.
As stated earlier, the status change ofS
j
andA
j
can be received from di®erent
channels. When the assessmentA
j
becomes available, one of the database servers
Fig. 4. Flow of submissions and assessments in load balancer.
Online Judge System: Requirements, Architecture, and Experiences13
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

stores it via the instance of VerdictUpdator. Based on the request of the Web server,
the status ofS
j
andA
j
is provided via the instance of StatusProvider. This channel is
used to provide the latest status quickly without referring to the database server.
The broadcaster is activated by the instance of StatusBroadcaster at various times.
For broadcasting, the load balancer provides information to the broadcaster when (i)
it receives a submission from the Web server and pushes it to the selected queue, (ii)
the submission is transmitted to the judge server, (iii) it receives the corresponding
assessment result after the judgment is completed, and (iv) the judge result is stored
in the database server. In this way, the status transitions between waiting, running,
and judged are reported.
In general, a conventional load balancer monitors the load of virtual machines in a
physical machine, and if the load of a virtual machine increases, the load is dis-
tributed by moving the task to another physical machine. The load balancer in OJS
balances the load by distributing the processing of the evaluation request (submitted
code) to multiple servers. Herein, we discuss the two main architectures for the load
balancing. The  ̄rst is an architecture with a single queue (e.g. [30]). In this archi-
tecture, the submitted program codes are inserted into a queue, timely requests are
received from each of multiple judge servers, and the submitted program codes are
retrieved from the queue and processed. The queue can be implemented as a priority
queue, and the user's waiting time can be controlled according to criteria. The
criteria can include arrival time, estimated waiting time, language, experience of the
user, and evaluation statistics of the problem. The second is a multiqueue archi-
tecture (e.g. [31]). In this architecture, a queue is managed for each judge server, and
the queue-in destination (i.e. judge server) is determined in advance according to the
criteria at the moment a submitted program code is received. In addition to the
criterion of the architecture with a single queue, the information of the individual
server such as CPU load, memory usage, I/O load, etc. can be considered. The
architecture with a single queue has the advantage that idling machines can pull
requests sequentially and process them in an optimal order. However, because locks
on a single queue occur frequently, the implementation and synchronization process
becomes more complex. On the other hand, the multiqueue architecture is a simple
and robust architecture that does not place a large load (lock) on a single queue.
Furthermore, it has the advantage of distributing the load when notifying the status
of the queue elements. However, because the actual processing time is not known
until the program code is executed and evaluated, it can be said that the impact of a
deviation from the predicted value is signi ̄cant.
Moreover, to deal with multiple test cases, the following parallel architectures can
be considered for an OJS. The  ̄rst is an architecture where a request occupies one
judge server until it completes all test cases (parallelization for requests). Multiple
requests can be performed exclusively within parallel machines. The advantage of
this architecture is that it is easy to manage submissions while maintaining consis-
tency, and there are few harmful e®ects on other submissions when problems occur.
However, it cannot make use of free judge servers, especially when there are only a
14Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

few requests. The second is an architecture where test case runs are distributed to
di®erent judge servers (parallelization for test cases). In other words, judgments for a
set of test cases are performed in parallel. The advantage of this architecture is that,
in principle, it enables e±cient parallel processing. However, this architecture
requires more communications between the load balancer and the corresponding
judge server, and it is somewhat di±cult to manage the distribution and collection
processes. In addition, we should consider maintaining fairness and consistency be-
cause test cases can be handled by di®erent machines, and other requests being
processed can be easily a®ected. Based on our policy described in Sec. 3, we employ
the former parallelization architecture. Because the latter parallelization also pro-
vides a promising option for dealing with test cases (such as contest platforms with
large submission amounts during a speci ̄c period), it will be a topic of our future
work.
5.2.Broadcaster
Because the noti ̄cation is realized by asynchronous communication, WebSocket is
employed for the push-type communication for broadcasting. Figure5shows the
software architecture with communication between the external systems, the
broadcaster, and the load balancer. When a client of the external system accesses
the broadcaster, the client system sends a request to the broadcaster through
WebSocket, and establishes a connection. After that, when a change occurs in the
load balancer, it sends the status to the broadcaster via a TCP connection. The
broadcaster waits for receiving status, and when a status arrives, the broadcaster
receives it through the internal TCP Server, which forwards the information to
another WebSocket Server. Then the broadcaster broadcasts the information to all
the connected clients through the already established WebSocket connections.
The system based on the above architecture needs to keep all connections from
the external systems and notify all when any status change occurs. Generally, such a
Fig. 5. Broadcaster architecture.
Online Judge System: Requirements, Architecture, and Experiences15
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

system can be under high load. Furthermore, the concurrent system requires so-
phisticated threading and shared data management. So, as the software architecture
to implement the broadcaster, we employed an Actor Model oriented to high per-
formance, fault tolerance and scalable systems. In the Actor Model, immutable
objects (actors) communicate each other by sending messages without shared
data [32].
Figure6shows a class diagram of the broadcaster. The classes Server, Server-
Handler and WebSocketHandler are instantiated as actors, and the main process is
performed based on communication by exchanging messages between them. The
WebSocketHandler is a class that is instantiated as an actor for each request of
WebSocket connection from the external system. The actor has roles to validate the
status and convert it to a speci ̄c format (e.g. JSON), and send the data to the
corresponding external system. Server is a class that is used to receive TCP con-
nection from the load balancer. After a TCP connection is established, it creates
an actor of ServerHandler class, then it broadcasts the data to the actors of
WebSocketHandler, each of which forwards the data to the corresponding external
system.
5.3.Judge cluster
The con ̄guration of a judge server is shown in Fig.7, where a host operating system
(OS) is installed on a dedicated hardware machine. Within the OS there are  ̄ve
modules (processes): observer, controller, launcher, executor, and judge. The ob-
server is the top administrator that can monitor all processes of the submission and
kills if there are threatening elements in running programs. The controller is pri-
marily responsible for bridging between the load balancer and the judge server. It
receives submissionS
j
and returns the corresponding assessmentA
j
. The launcher
prepares the given solution code for execution, then the executor is launched from the
launcher to execute the solution code. Finally, the judge evaluates the output and
behavior of the user program. Since problems can occur in the launcher, executor,
and judge phases due to the handling of the user program, the main role of the
observer is to monitor the processes in these three phases (dashed arrows in Fig.7).
Speci ̄cally, it monitors the compiler process in the launcher phase, the process of
code:exein the executor phase, and thecheckerandreactiveprocesses in the judge
Fig. 6. Class diagram of broadcaster.
16Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

phase. If processes with unusually long execution times are detected, they are killed
by the observer.
The required data and programs are installed into the OS in advance, as shown in
the bottom part of Fig.7. Judge data is a set ofJ
p
, each of which is deployed by the
judge master. The compilers/runtime is a compilation/execution environment for a
given solution code in available programming languages. Measures is a set of pro-
grams used by the judge server that include available OS commands, while work-
space refers to a designated location where  ̄les generated and read by the modules
are managed. The elements bounded by the dotted lines in Fig.7can be virtualized
in a dedicated container. The  ̄rewall is con ̄gured so that the judge server can only
communicate with the load balancer and the judge master. The host OS can com-
municate with the load balancer via the controller. Further, the judge data is
deployed by the judge master via another secure communications. All other com-
munications are blocked by the  ̄rewall.
The sequence diagram for describing the assessment for a submission request is
shown in Fig.8, in which data resources and processes presented above are involved.
The controller waits for a request from the instance of SenderConnection. After
receiving a request, the launcher is activated. Simultaneously, the data for the so-
lution codesourceand the associated metrics provided byS
j
are set as data  ̄les. If
Fig. 7. Judge server architecture.
Online Judge System: Requirements, Architecture, and Experiences17
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

Fig. 8. Judge server sequence diagram.
18Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

the request is aCUSTOMjudge, the user-de ̄ned input and output are stored in the
workspace and they are used for the judge.
Next, the launcher attempts to compile the solution codesource(if necessary). If
the attempt fails, an error messagemsgis generated, and the process is sent back to
the controller. In the last phase, the controller sends the error messagemsgto the
instance of SenderConnection. In contrast, if the launcher process is successful,
according to the number of test casesncases, the generated codecode:exeis executed
by the executor under resource limitations. At this point, the executor generates a
runtime signal and a messagemsgif an error occurs during execution, otherwise, it
generates an output  ̄leoutputfor each test case. The process is terminated by the
executor or the observer when at least one of the resources exceeds its limit. Then the
judge is called to assess the output using the related data and program. Finally, when
the solution code is rejected or passes all test cases, the control is returned to the
controller with a summary of the verdictA
j
## .
The presented OJS is characterized by the fact that the process of judging is not a
monolithic piece of software, but is realized by multiple script programs on the OS.
These scripts share  ̄les on the OS and carry out the processes sequentially. For each
process, they utilize commands and resource management methods provided by the
OS. The only resident processes in the judge server are the controller, which accepts
requests, and the observer, which monitors the entire process. Moreover, this ar-
chitecture is highly susceptible to the commands and management methods provided
by the OS, and the reliability of the OS a®ects the reliability of the judge system.
## 6. Experiences
In this section, we  ̄rst present the hardware setup, software technologies, and
applications of the presented OJS in the operational period as reference data for
discussion. Then, we evaluate the OJS based on the real experience of the last decade
through statistical analysis and experiments. Challenges for developing and oper-
ating OJS will also be discussed.
6.1.Overview of hardware setting
The load balancer is a system that is subject to many loads, such as the process of
handling browsing requests from users, the multithreaded process that occurs for
each submission request, and managing multiple queues with many submissions lined
up. Therefore, it is desirable to have a high-spec machine that takes system utili-
zation into account. For more on the judge servers, while the submitted program is
being compiled, executed, and evaluated on the judge server, these processes basi-
cally monopolize the resources exclusively. This is because other user programs are
not running at the same time on the same server (actually, the processes of the user
program and the judge modules run in parallel on the same machine). Therefore, the
CPU and memory are of su±cient speci ̄cation to evaluate a single program that
Online Judge System: Requirements, Architecture, and Experiences19
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

solves the problems provided by AOJ. As the judge servers age, they have been
updated to higher specs considering the newness and the consistency. Details of the
hardware setting of the operational period are available system information provided
by the AOJ site [11].
The load balancer and each judge server have their global IP address of the same
segment. The  ̄rewall has been established so that each judge server can communi-
cate with only the load balancer via speci ̄c ports. During the operational period, the
load balancer has been implemented in such a way that each judge server is dedicated
to assess given codes in several programming languages. This means that programs of
the same version of programming language have been assessed by the same judge
server.
6.2.Overview of software technologies
The AOJ system is implemented and operated with various technologies for di®erent
components based on the presented architecture. Java technologies are used for the
web server, load balancer, and broadcaster. Speci ̄cally, the web server has been
implemented in Apache Tomcat (before 2018) and Spring Framework (after 2018),
the load balancer is a pure Java application, and the broadcaster is implemented in
Scala. On the other hand, the modules of the judge server are executed in scripting
languages. Speci ̄cally, the observer is executed in Python, and the controller,
launcher, executor, and judge are running in Perl.
6.3.Applications
In this section, we brie°y present the practical applications of our AOJ system in
various programming activities including practices, challenges, and academia. The
AOJ system has automatically evaluated more than 6 million program codes in the
last 10 years. In addition, more than 100 o±cial programming contests were held and
more than 3000 virtual competitions and exercises were conducted during this long
time period. At the same time, the AOJ system is being used as an academic tool in
programming-related courses at the University of Aizu. Most importantly, the AOJ
system has been in operation basically 24 h a day for 10 years for programming
practices and contributes to programming learning. These statistics con ̄rmed the
performance of the AOJ system and at the same time the building theory of the
system which was primarily demonstrated in this study.
6.4.Operation ratio
Table1shows the availability of the OJS by year. One of the indicators of avail-
ability is the history of time stamps that the judge servers actually evaluated given
codes. We, therefore, divided a day into 24 time equivalent compartments, and
counted the compartments where at least a single judge was performed. Maint. is the
20Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

time (hours) that the OJS did not perform evaluations due to planned maintenance
or the power outages (for periodic planned power outages of university facilities),
while Idling is the time (hours) that the OJS did not perform evaluations because of
other reasons.
Working and Total is the number of hours of actual operation and the total
number of planned hours in a year, respectively. Ratio is the annual percentage of
hours that OJS performed assessments successfully. From our observation, the idling
is mostly caused by a poor network environment and hardware failure. In addition,
Idling in the initial period of operation is likely to be a®ected by the time when no
submissions were received from users for a long period of time.
6.5.Submission status
Here, we investigate the waiting time of each submission, which is the di®erence
between the time the instance of ReceiverConnection received the submission and
the time the instance of SenderConnection received the corresponding evaluation
results. So, in this statistics, the time cost between the external system and the
broadcaster (through the Web server and the broadcaster, respectively) is not con-
sidered (it can be accomplished in a moment).
Table2and Fig.9show the total number of submissions as well as average,
median, and mode values of waiting time (in second (s)) for each year (the data for
2011 is excluded because accurate statistics cannot be obtained due to missing data).
The mean, median, and mode data are the combined values of all servers (pro-
gramming languages). Improvements in latency can be seen in each of 2013 and 2018.
The improvement between 2012 and 2013 can be attributed mainly to the paralle-
lization of judge servers and the improvement of the speci ̄cations of the server itself.
Further, before 2018, the executor was designed to execute the submitted program
for a certain period of time even if it falls into TLE. On the other hand, after 2018,
the speci ̄cation was changed to execute the submitted program only for a time
Table 1.  Operating ratio of OJS.
## Year   Maint.   Idling   Working   Total   Ratio (%)
## 2011714248265868995.12
## 2012172968471876796.62
## 2013511058604870998.79
## 2014451278588871598.54
## 201545778638871599.11
## 201648658671873699.25
## 201747408673871399.54
## 201842318687871899.64
## 201941288691871999.67
## 20204348737874199.95
## 20214378710871799.91
Online Judge System: Requirements, Architecture, and Experiences21
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

based ontimeLimde ̄ned inM
p
. Here the results show that the average waiting
time is linear despite the increasing number of submissions.
The mode value indicates the interval with the most frequent value, counting the
corresponding waiting time in 0.1 s increments. Table2shows that most assessments
are completed between 0.10 and 0.19 s. Figure10shows the number of submissions
evaluated for each waiting time interval, delimited by a tenth of a second.
Table 2.  Submission and waiting time of the OJS.
## Year   Submissions   Average   Median   Mode
## 2011169123
## 20122234682.9161.3821.1
## 20132856461.7280.2800.1
## 20143270741.8800.3180.1
## 20154518031.8580.3470.2
## 20165137471.7100.3140.1
## 20175206421.9170.3230.1
## 20186497681.3890.3420.1
## 20197611611.4130.3510.1
## 202010146831.6300.3410.1
## 202110597561.1690.2830.1
Fig. 10. Submission numbers for each interval of waiting submissions.
(a) Year-wise submissions(b) Year-wise waiting time
Fig. 9. Year-wise total submissions and waiting time trends.
22Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

Noteworthy, around one million submissions were assessed in less than 0.2 s. Al-
though there is a long tail of submissions that have longer waiting times, approxi-
mately 79.11% of submissions were processed very quickly within 1 s.
Table3shows the maximum number of codes and test cases assessed per second,
minute, and hour, respectively. The statistical results show the highest performance
of our OJS in real-world operation over the decade. Note that the presented per-
formance  has  been  achieved  by  the  parallel  processing  with  several  judge
servers available during the operation period. Although the result depends on the
period which obtained the maximum value, it shows operational achievements
(the potential performance is discussed in the following section).
6.6.Performance evaluation of judge server through stress tests
In fact, our OJS is capable of processing much more submissions than the results
shown in Table3. This result is due to the idling time during the operation period.
So, we performed additional experiments to show the potential performance of a
single judge server through stress tests. The experiment was conducted with a special
imitator of the load balancer which sends a set of program codes to a judge server so
that it can evaluate a given code exclusively.
We performed the stress tests for di®erent problems 1, 2 and 3 which can be solved
byOð1Þalgorithm. To analyze the features of the judge server, we selected three
di®erent problems which have a di®erent number of test cases, 1, 4 and 10, respec-
tively. We also consider di®erent programming languages C, Cþþ, Java, and Python
which are currently the most popular languages and have di®erent mechanisms of
execution by binary  ̄les generated by compilers, the virtual machine, and the in-
terpreter. Tables4,5, and6show the potential performance obtained by the stress
tests (note that the value of hours is not obtained by actual measurement, but by
multiplying the corresponding minutes by 60). Overall, the judge server assessed
problems with many test cases despite the small number of codes. In other words,
Table 3.  Maximum performance based on evaluated codes and test cases.
LangTotal number of codes in:Total number of test cases in:
## Seconds    Minutes    Hours    Seconds    Minutes    Hours
## All71251600294253519953
Table 4.  Potential performance by stress tests: Problem 1.
LangTotal number of codes in:Total number of test cases in:
## Seconds   Minutes   Hours   Seconds   Minutes   Hours
## C12626375601262637560
## Cþþ10526315601053631560
## Java3151906031519060
## Python13756453601375645360
Online Judge System: Requirements, Architecture, and Experiences23
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

judges for problems with more test cases can process more test cases in total,
although the number of codes that can be processed is smaller. This is because the
switching of test cases is completed within the judge server and no communication
between the judge servers and others. As mentioned above, the presented perfor-
mance was achieved by a single judge server. In principle, ifNjudge servers are
deployed, their performance will beNtimes higher.
6.7.User experiences in receiving a judge verdict
While it would be ideal to evaluate the performance of the presented system, it is not
easy to compare the performance of the AOJ with that of other operational OJSs.
This is because fairness cannot be maintained due to the environment of hardware
power and network latency. On the other hand, although it is not appropriate to
claim the performance of a system by means of a subjective questionnaire, it is
considered to be a way of evaluation. Therefore, we report the results of question-
naire surveys of AOJ users. AOJ has been used for eight years in exercises for the
Algorithm and Data Structure course, mainly for  ̄rst- and second-year students at
the University of Aizu, and the programs created in the exercises are judged by AOJ.
Here, we report the results of questionnaire survey obtained from the students of the
exercises conducted in 2018. The questionnaire includes a variety of questions about
the usefulness of the AOJ for learning the subject. One of the items in the ques-
tionnaire is a question about the validity of the waiting time until the judge  ̄nishes.
More  speci ̄cally,  users  were  asked  about  their  experiences  with  the
\waiting time" between the time their submitted programs were processed and the
time they received responses from the judge. The responses are on a four-point scale
ofvery fast,fast,slow,andvery slowwith no intermediate values allowed. In 2018,
Table 5.  Potential performance by stress tests: Problem 2.
LangTotal number of codes in:Total number of test cases in:
## Seconds   Minutes   Hours   Seconds   Minutes   Hours
## C84142484032165699360
## Cþþ73722232028148889280
## Java310160601240424240
## Python93972382036158895280
Table 6.  Potential performance by stress tests: Problem 3.
LangTotal number of codes in:Total number of test cases in:
## Seconds   Minutes   Hours   Seconds   Minutes   Hours
## C523914340502390143400
## Cþþ422513500402250135000
## Java15533001055033000
## Python419811880401980118800
24Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

537 students took this course, and valid responses were received from 370 students.
The result of the questionnaire is shown in Table7. This qualitative evaluation shows
that most of the students were satis ̄ed with the judges' response time.
6.8.Discussion and challenges
In this section, we discuss the non-functional requirements of the presented OJS
considering the architecture, experiences, and experiments described above. Re-
search challenges for the development and operation regarding the requirements are
also discussed.
Immediacy (good).The mean, median, and mode data of the actual waiting
time during the operation period shown in Table2indicate the high immediacy of
the presented OJS. However, this statistical information of waiting time depends on
the learning contents of the AOJ, such as the computational complexity of the
algorithm in the submitted program codes, the number of cases, constraints
(timeLimandmemLim) speci ̄ed to each problem, and the content of the problem,
as well as the level of the users and the cost-e®ectiveness of the server. In addition,
the same algorithm can vary in execution time depending on the programming
language (e.g. scripting languages are generally slower). Thus, the data is indicative
of operational experience. Moreover, the results of the additional measurements
shown in Tables4,5, and6assume program codes using theOð1Þalgorithm. In other
words, they represent pure judge performance per judge server, which can be said to
indicate the high immediacy of the OJS. This is because the payloads of the load
balancer and judge server are lightweight, and the script programs of the judge server
are also lightweight. The immediacy is also supported by the user experience. Fur-
thermore, since the judge servers are multi-core machines, it may actually be better
to run multiple user programs simultaneously on a single server. As machines have
become increasingly multi-core in recent years, an additional experiment is needed to
achieve e±cient parallelization (AOJ's judge server has also increased in number of
cores with each successive generation, from the  ̄rst generation's 2 to 6). This ex-
periment should verify its e®ectiveness from the perspective of parallelization e±-
ciency, in°uence on the consistency, and resource sharing (cores,  ̄le access,
command execution, network communication, etc.) of the judge module process for
multiple requests.
Availability (good).Table1shows that despite the increase in submissions the
availability of this OJS over 10 years has been maintained at almost 99% or higher.
In other words, the outage rate is extremely low and its high availability is observed.
Table  7.  Results  of  judge  response
evaluation based on questionnaire.
Very fast    Fast    Slow   Very slow
## 53.7%42.9%   2.9%0.2%
Online Judge System: Requirements, Architecture, and Experiences25
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

As shown in the robustness feature below, the architecture of the load balancer and
the judge server have succeeded in keeping the system running normally at all times.
As long as no unexpected hardware or network failures occur, the presented OJS can
continue to provide stable services.
Robustness (good).In addition to the availability mentioned above, Table1
also shows the high robustness of the OJS. As in a typical system, the judge servers
are parallelized so that they can compensate for each other in case of failure. Each
judge server has process management by the OS and resource limit settings to keep
the machine in a normal state even when arbitrary submitted programs are re-
peatedly executed. In addition, running the observer all the time deters the system
from freezing. In the load balancer, each judge server has its own queue, which avoids
excessive locking of the submission queue and provides stable control. However, it is
necessary to consider the load of broadcast and to test the performance under the
assumption of large-scale submission concentration.
Sustainability (good).In the past 10 years, there have been no major changes
in the software architecture of the load balancer and the judge server, and these have
been running continuously and stably (although major changes have been made to
the web server and the client side of the AOJ). This architecture is based on script
programs that control the OS commands after the OS has been deployed and con-
̄gured, and the data format and mechanism are immutable. In other words, there is
little dependence on speci ̄c technologies for development and operation. Although it
is necessary to check and test if there are any changes in the speci ̄cations of the
commands used when updating the OS, these features show that we can guarantee
the sustainability of the system in the future.
Correctness (average).One useful stage for verifying the correctness of the
system is a competition that requires more critical judging. In the past 10 years, more
than 100 o±cial contests have been held using the presented OJS, and no judges have
been rejudged due to the failure of the judge system. This indicates that our OJS has
a certain degree of correctness. Further, it will be necessary to conduct demonstra-
tion experiments assuming a large number of participants who will place a heavy
load on the load balancer and the judge servers. On the other hand, the non-
functional requirements such as correctness depend on the implementation and
con ̄guration of the OS commands, and if there is a defect in the OS program or
execution process, it may not be possible to perform a proper evaluation. Currently,
by detecting outliers, wrong decisions are avoided. For more critical applications, it
would be better to consider measures such as double-checking.
Security (average).Static analysis of user programs is not a feasible method to
detect attacks. In the presented OJS, a sandbox is constructed on the OS side by
setting up a  ̄rewall, restricting the executing user, and restricting access to im-
portant  ̄les to ensure minimum security. In general, enhanced security a®ects im-
mediacy. In our experiment, we prioritized immediacy and did not implement a
sandbox for the submodules shown by the dotted lines in Fig.7. It will be necessary
26Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

to research a mechanism to minimize the impact of containerizing these submodules.
In addition, as shown below, the priority of consistency is at the expense of security.
Scalability (average).The architecture of the load balancer, in which each
queue is managed by a single thread, is scalable and can be easily scaled out by
adding more physical servers. Even when the utilization rate of the entire service
increases, this scalability can reduce the impact on the overall immediacy. In addi-
tion, scalability can maintain robustness even when the judge server fails. Broad-
caster is also scalable to support simultaneous connections from the viewpoint of
software architecture by the Actor model (cost-e®ectiveness). However, it may be
necessary to have a stress test of the queue with a large number of users as well as
judge servers.
Transparency (average).The service side or server side using the presented
OJS can obtain the status of submissions from di®erent channels from the load
balancer (web server) and from the channel of asynchronous noti ̄cation from the
broadcaster. In addition, it can get the test cases that are exactly used for the judge
through the judge master. Then, these are the high transparency of the OJS. In
principle, the transparency can be further improved by allowing communication
between the broadcaster and each judge server, and asynchronously notifying the
evaluation results of each test case process sequentially. In this case, however, se-
curity and immediacy would be sacri ̄ced, and server expansion and experimentation
would be necessary to take the load into account.
Consistency (average).In the operation of the presented OJS, updates of
hardware, OS and language versions are done after a certain period of time. This is to
maintain the consistency of the evaluation results. However, such an operation is
detrimental to security and newness, so it is necessary to determine when to update
depending on the situation. The user's program is executed exclusively, to the ex-
clusion of other services, so that it can make exclusive use of the server's resources.
Moreover, there is some °uctuation in the measurement of CPU time and memory
usage of user programs because the OJS uses OS commands and resources.
Newness (poor).The presented OJS sacri ̄ces newness because it prioritizes
consistency. This is unfriendly to users who want to try the latest version. Moreover,
be aware that this will a®ect security. However, it is possible to achieve both con-
sistency, newness, and security by de ̄ning rules for handling service statistics and
results, and then implementing a mechanism for automatic rejudge on a regular
basis.
Portability (poor).When deploying the presented OJS, a special con ̄guration
is required to form a sandbox, so the portability of the judge system is not high. In
addition, when the OS is updated, commands need to be modi ̄ed. If the platform is
changed, the software version has to be upgraded. Moreover, as with security, this
problem can be improved by containerization.
Economic (poor).During the operation of the presented OJS, the hardware is
always running, and the idling time of the judge servers is long and not energy
e±cient. Scaling by planning and congestion prediction, and dynamic assignment
Online Judge System: Requirements, Architecture, and Experiences27
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

mechanisms need to be studied. Related to this, it would also be interesting to predict
the CPU load required by the submitted program code and using it for the load
balancing.
## 7. Conclusion
Although OJSs have many attractive applications, the theories for constructing
them have not been su±ciently discussed in previous studies. In this paper, we have
identi ̄ed the functional and non-functional requirements of an OJS, and then
clari ̄ed the architecture and algorithms of our own OJS. Speci ̄cally, we demon-
strated the relationship of the major components, software architecture, data °ow,
threading, and sequencing mechanism in detail. In addition, the system was vali-
dated through 10 years of operation and experience, as well as through experiments
and user feedback. This OJS is dedicated to immediacy, robustness, availability,
transparency, consistency, and sustainability among the non-functional require-
ments. The study provides a guideline for third parties to build an OJS as well as
presents a construction theory for further discussion and evolution of OJS. Speci ̄-
cally, we have presented details of the load balancer, broadcasting system, and judge
system with some research issues. These issues include
.di®erent types of queuing architectures and algorithms to distribute the load of a
large number of submission requests,
.di®erent types of parallel architectures of judge servers to process the requests
exclusively and e±ciently,
.an architecture and model to notify all audiences in real time with status infor-
mation of the requests, and
.a software con ̄guration and judging sequence to process the solution code.
They are related to the theory of the construction of OJS, which must continue to
evolve to meet the recent increasing number of users and requirements. This paper
presented one practical theory for each issue. However, although this paper also
clari ̄ed and analyzed non-functional requirements of OJS, satisfying them with
trade-o®s simultaneously is not easy, and it will be a subject of future research.
## Acknowledgment
This research work was supported by the Japan Society for the Promotion of Science
(JSPS) KAKENHI (Grant Number 19K12252).
## References
-  S. Wasik, M. Antczak, J. Badura, A. Laskowski and T. Sternal, A survey on online judge
systems and their applications,ACM Comput. Surv.51(1) (2018) 1–34.
28Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

-  J. Petit, S. Roura, J. Carmona, J. Cortadella, J. Duch, O. Gimnez, A. Mani, J. Mas, E.
Rodrguez-Carbonell, E. Rubio, E. D. S. Pedro, D. Venkataramani, Jutge.org: Char-
acteristics and experiences,IEEE Trans. Learn. Technol.11(3) (2018) 321–333.
-  K. Georgouli and P. Guerreiro, Incorporating an automatic judge into blended learning
programming activities,Int. Conf. Web-Based Learning, Vol. 6483, 2010, pp. 81–90.
-  C. Yi, S. Feng and Z. Gong, A comparison of sandbox technologies used in online judge
systems,Appl. Mech. Mater.490–491(2014) 1201–1204.
-  M. M. Rahman, Y. Watanobe and K. Nakamura, Source code assessment and classi ̄-
cation based on estimated error probability using attentive LSTM language model and its
application in programming education,Appl. Sci.10(2020) 2973.
-  T. Matsumoto, Y. Watanobe and K. Nakamura, A model with iterative trials for cor-
recting logic errors in source code,Appl. Sci.11(2021) 4755.
-  K. Terada and Y. Watanobe, Code completion for programming education based on
recurrent neural network,2019 IEEE 11th Int. Workshop Computational Intelligence and
Applications, 2019, pp. 109–114.
-  M. M. Rahman, Y. Watanobe, R. U. Kiran, T. C. Thang and I. Paik, Impact of practical
skills on academic performance: A data-driven analysis,IEEE Access9(2021) 139975–
139993, https://doi.org/10.1109/ACCESS.2021.3119145.
-  B. Jiang, S. Wu, C. Yin and H. Zhang, Knowledge tracing within single programming
practice using problem-solving process data,IEEE Trans. Learn. Technol.13(4) (2020)
## 822–832.
-  M. M. Rahman, Y. Watanobe, T. Matsumoto, R. U. Kiran and K. Nakamura, Educa-
tional data mining to support programming learning using problem-solving data,IEEE
Access10(2022) 26186–26202, https://doi.org/10.1109/ACCESS.2022.3157288.
-  Y. Watanobe, Aizu online judge, 2018, https://onlinejudge.u-aizu.ac.jp.
-  R. Puri, D. Kung, G. Janssen, W. Zhang, G. Domeniconi, V. Zolotov, J. Dolby, J. Chen,
M. Choudhury, L. Decker, V. Thost, L. Buratti, S. Pujar and U. Finkler, Project codenet:
A large-scale AI for code dataset for learning a diversity of coding tasks, arXiv:abs/
## 2105.12655.
-  Online judge, https://onlinejudge.org/.
-  Sphere online judge, https://www.spoj.com/.
-  M. A. Revilla, S. Manzoor and R. Liu, Competitive learning in informatics: The UVa
online judge experience,Olymp. Inform.2(2018) 131–148.
-  R. Queirs and J. P. Leal, De ̄ning programming problems as learning objects, inProc.
Int. Conf. Computer Education and Instructional Technology, 2009, pp. 28–30.
-  A. Kosowski, M. Mala ̄ejski and T. Noinski, Application of an online judge & contester
system in academic tuition, inInt. Conf. Web-Based Learning, 2007, pp. 343–354.
-  Contest management system (CMS), https://cms-dev.github.io/.
-  Domjudge, https://www.domjudge.org/.
-  H. Yibo, Z. Zhang, B. Yuan, H. Bi, M. N. Shahzad and L. Liu, An experimental online
judge system based on docker container for learning and teaching assistance, in2019
IEEE SmartWorld, Ubiquitous Intelligence Computing, Advanced Trusted Computing,
Scalable Computing Communications, Cloud Big Data Computing, Internet of People and
Smart City Innovation, 2019, pp. 1462–1467.
-  H. Liang, C. Chen, X. Zhong and Y. Chen, Design and implementation of online auto-
matic judging system,IOP Conf. Ser. Earth Environ. Sci.69(2017) 012091, https://doi.
org/10.1088/1755-1315/69/1/012091.
-  M. Wang, W. Han and W. Chen, MetaOJ: A massive distributed online judge system,
Tsinghua  Sci.  Technol.26(4)  (2021)  548–557,  https://doi.org/10.26599/TST.
## 2020.9010016.
Online Judge System: Requirements, Architecture, and Experiences29
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.

-  S. R. Labs, Online compilers and programming challenges APIs-sphere engine, https://
sphere-engine.com.
-  A. Prologin, camisole, https://camisole.prologin.org.
-  H. Z. Doilovic and I. Mekterovic, Robust and scalable online code execution system, in
2020 43rd Int. Convention Information, Communication and Electronic Technology,
2020, pp. 1627–1632.
-  Judge0 api source code, https://github.com/judge0/api.
-  I. Mekterovic, L. Brkic, B. Milainovic and M. Baranovic, Building a comprehensive au-
tomated programming assessment system,IEEE Access8(2020) 81154–81172.
-  R. Conejo, B. Barros and M. F. Bertoa, Automated assessment of complex programming
tasks using SIETTE,IEEE Trans. Learn. Technol.12(4) (2019) 470–484, https://doi.
org/10.1109/TLT.2018.2876249.
-  Y. Watanobe, M. M. Rahman, U. K. Rage, R. Penugonda, Online automatic assessment
system for program code: Architecture and experiences, in:Advances and Trends in Arti-
̄cial Intelligence. From Theory to Practice, Springer International Publishing, Cham,
2021, pp. 272–283.
-  L. Fangzheng, J. Shaohai and W. Yanrong, A kind of algorithm contest online judge of
high concurrent, Shanghai Luogu Network Technology Co Ltd., CN109508293A (2019).
-  D. S. Reiner, G. M. Ericson and E. StPierre, Architecture for using a model-based ap-
proach for managing resources in a networked environment, EMC Corp US10303517B1
## (2011).
-  P. Haller and M. Odersky, Actors that unify threads and events, inCoordination Models
and Languages 9th Int. Conf., COORDINATION, Vol. 4467, 2007, pp. 171–190.
30Y. Watanobe et al.
Int. J. Soft. Eng. Knowl. Eng. Downloaded from www.worldscientific.com
by UNIVERSITY OF AIZU on 07/05/22. Re-use and distribution is strictly not permitted, except for Open Access articles.