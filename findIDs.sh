#!/usr/bin/bash

inputMessages=$1

grepOutput=`grep $inputMessages IDS`
if [[ -z $grepOutput ]];
then
    echo $inputMessages >> IDS
else
    echo "Already exists in file"
fi


