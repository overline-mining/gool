#!/bin/bash

BUILD_ARCH=""

case $(lscpu | grep Architecture | awk '{print $2}') in

  x86_64)
    export BUILD_ARCH=x86_64-linux-gnu
  ;;

  armv7*)
    export BUILD_ARCH=arm-linux-gnueabihf
  ;;

  aarch64*)
    export BUILD_ARCH=aarch64-linux-gnu
  ;;

  *)
    exit 1
  ;;

esac

echo $BUILD_ARCH
