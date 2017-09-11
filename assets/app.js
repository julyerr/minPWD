var app = angular.module('DockerPlay', []);

  app.controller('PlayController', ['$scope', '$log', '$timeout', '$window',function ($scope, $log, $timeout,$window) {
   var terminalContainer = document.getElementById('terminal-container');
      var term = new Terminal({
        cursorBlink: false
      });
      term.attachCustomKeydownHandler(function(e) {
        // Ctrl + Alt + C
        if (e.ctrlKey && e.altKey && (e.keyCode == 67)) {
          document.execCommand('copy');
          return false;
        }
      });
      term.on('data', function(d) {
	console.log("sending "+d)
     	$scope.socket.emit('terminal in', d);
	 });
      
      $scope.term = term ;
      
      term.open(terminalContainer);


	 setTimeout(function() {
        $scope.resize(term.proposeGeometry());
      }, 4);

    var socket = io({ path:  window.location.pathname+'/ws'});
    socket.on('connect_error', function() {
      console.log("disconnected")
    });

    socket.on('connect', function() {
      console.log("connected")
    });

    socket.on('terminal out', function(data) {
	console.log("receiving data ",data)
        $scope.term.write(data)
      });

    socket.on('viewport resize', function(cols, rows) {
      // viewport has changed, we need to resize all terminals
	console.log("view resize, cols rows ",cols,rows)
      $scope.term.resize(cols, rows);
    });
          
    $scope.socket = socket;

    $scope.resize = function(geometry) {
     $scope.socket.emit('viewport resize', geometry.cols, geometry.rows);
    }
}]);
