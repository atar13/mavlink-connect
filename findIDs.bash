#! /bin/bash


# read inputMessages 
inputMessages=$1

grepOutput=`grep $inputMessages IDS`
echo $grepOutput
if [[ -z $grepOutput ]];
then
    echo $inputMessages >> IDS
else
    echo "Already exists in file"
fi


