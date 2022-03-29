#!/bin/bash

BUILD_ARCH=""

case $(lscpu | grep Architecture | awk '{print $2}') in

  x86_64)
    export BUILD_ARCH=amd64
  ;;

  armv7*)
    export BUILD_ARCH=arm7
  ;;

  aarch64*)
    export BUILD_ARCH=arm64
  ;;
  
  i686)
    export BUILD_ARCH=386
  ;;

  *)
    exit 1
  ;;

esac

echo $BUILD_ARCH
