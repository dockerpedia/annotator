# DockerPedia Annotator

Disclamer: This note is work-in-progress and will be progressively updated

Experiment reproducibility is the ability to run an experiment with the introduction of changes to it. To allow reproducibility, the scientific community encourages researchers to publish descriptions of the these experiments. However, these recommendations do not include an automated way for creating such descriptions: normally scientists have to annotate their experiments in a semi automated way. In this paper we propose a system to automatically describe computational environments used in in-silico experiments. We propose to use Operating System (OS) virtualization (containerization) for distributing software experiments throughout software images and an annotation system that will allow to describe these software images. The images are a minimal version of an OS (container) that allow the deployment of multiple isolated software packages within it. 
