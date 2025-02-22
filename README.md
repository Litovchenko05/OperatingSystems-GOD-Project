# GOD | Operating Systems Project - UTN FRBA
## 2nd Semester - 2024
📃 GOD: [Statement](https://docs.google.com/document/d/1HSZ14tk7IOfkOf-7ni0Wa6wnKZClEQA7zZyv-h0EZAY/edit?tab=t.0)

💡 This project is developed in Golang, utilizing APIs for inter-module communication. It is executed on a Linux environment and designed for a distributed system setup, replicating key OS functionalities such as process management and system calls.

# Project Structure
The project is organized into the following modules:

- **kernel/**: Manages process scheduling and coordination.
- **memoria/**: Handles system memory management.
- **filesystem/**: Implements file system operations.
- **cpu/**: Executes process instructions and manages interactions with memory and the kernel.

# Prerequisites
To compile and run this project, you need to have installed:

- **Go**: Version 1.22 or higher.

# Installation
Follow these steps to set up and run the project on your local machine:

## Clone this repository:
```sh
git clone https://github.com/Litovchenko05/OperatingSystems-GOD-Project.git  
cd OperatingSystems-GOD-Project  
```

## Compile the modules:
Navigate to each module directory and run:
```sh
go build  
```
This will generate the corresponding executables in each module.

# Execution
To start the operating system emulation:

## Start the memory module:
```sh
./memoria/memoria  
```

## Start the file system module:
```sh
./filesystem/filesystem  
```

## Start the CPU module:
```sh
./cpu/cpu  
```

## Start the kernel:
```sh
./kernel/kernel TEST-NAME  
