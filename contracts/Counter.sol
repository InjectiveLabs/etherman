pragma solidity ^0.6.0;

 contract Counter {
   uint256 counter = 0;

   event Increased(address who, uint256 oldValue, uint256 newValue);
   event Decreased(address who, uint256 oldValue, uint256 newValue);

   function add() public {
     emit Increased(msg.sender, counter, counter+1);
     counter++;
   }

   function addValue(uint256 value) public {
     emit Increased(msg.sender, counter, counter+value);
     counter = counter + value;
   }

   function subtract() public {
     emit Decreased(msg.sender, counter, counter-1);
     counter--;
   }

   function subtractValue(uint256 value) public {
     emit Decreased(msg.sender, counter, counter-value);
     counter = counter - value;
   }

   function getCounter() public view returns (uint256) {
     return counter;
   }
}
