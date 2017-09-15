(function() {
    'use strict';

    var app = angular.module('DockerPlay', ['ngMaterial']);

    app.controller('PlayController', ['$scope', '$log', '$http', '$location', '$timeout', '$mdDialog', '$window', 'TerminalService', 'KeyboardShortcutService', 'InstanceService', function($scope, $log, $http, $location, $timeout, $mdDialog, $window, TerminalService, KeyboardShortcutService, InstanceService) {
        $scope.sessionId = getCurrentSessionId();
        $scope.instances = [];
        $scope.idx = {};
        $scope.selectedInstance = null;
        $scope.isAlive = true;
        $scope.connected = true;
        $scope.isInstanceBeingCreated = false;
        $scope.newInstanceBtnText = '+ Add new instance';
        $scope.deleteInstanceBtnText = 'Delete';
        $scope.isInstanceBeingDeleted = false;
        $scope.storeSessionBtnText = 'Store';
        $scope.isSessionBeingStoring = false;
        $scope.resumeSessionBtnText = 'Resume';
        $scope.isSessionBeingResuming = false;
        $scope.IsTeacher = false;
        $scope.username = '';
        $scope.experiment='';
        $scope.sessionContent = '';
        $scope.sessions = [];
        $scope.sessionToResume=function (value) {
            if (value !== undefined){
                localStorage.setItem("sessionToResume",value);
                return
            }
            var session = localStorage.getItem("sessionToResume");
            if (session == null){
                return null
            }
            console.log("sessionToResume: "+localStorage.getItem("sessionToResume"))
            return session;
        }

        var selectedKeyboardShortcuts = KeyboardShortcutService.getCurrentShortcuts();

        angular.element($window).bind('resize', function() {
            if ($scope.selectedInstance) {
                $scope.resize($scope.selectedInstance.term.proposeGeometry());
            }
        });

        $scope.$on("settings:shortcutsSelected", function(e, preset) {
            selectedKeyboardShortcuts = preset;
        });

        function getCurrentSessionId() {
            return window.location.pathname.replace('/p/', '');
        }
        $scope.showAlert = function(title, content, parent) {
            $mdDialog.show(
                $mdDialog.alert()
                    .parent(angular.element(document.querySelector(parent || '#popupContainer')))
                    .clickOutsideToClose(true)
                    .title(title)
                    .textContent(content)
                    .ok('Got it!')
            );
        }

        $scope.resize = function(geometry) {
            $scope.socket.emit('viewport resize', geometry.cols, geometry.rows);
        }

        KeyboardShortcutService.setResizeFunc($scope.resize);

        $scope.closeSession = function() {
            // Remove alert before closing browser tab
            window.onbeforeunload = null;
            $scope.socket.emit('session close',$scope.username);
        }

        $scope.upsertInstance = function(info) {
            var i = info;
            if (!$scope.idx[i.name]) {
                $scope.instances.push(i);
                i.buffer = '';
                $scope.idx[i.name] = i;
            } else {
                $scope.idx[i.name].ip = i.ip;
                $scope.idx[i.name].hostname = i.hostname;
            }
            return $scope.idx[i.name];
        }


        $scope.showPrompt = function(cb) {
            // Appending dialog to document.body to cover sidenav in docs app
            var confirm = $mdDialog.prompt()
                .title('Please add comment to the session stored')
                .placeholder('Session')
                .ariaLabel('Session')
                .initialValue('session is stored for ...')
                // .required(true)
                .ok('Okay!');

            $mdDialog.show(confirm).then(function(result) {
                $scope.sessionContent = result;
                console.log("session store content: "+$scope.sessionContent)
                if (cb){
                    cb()
                }
            }, function() {
            });
        };
        function storeSession() {
            updateSessionBtnState(true);
            $http({
                method: 'POST',
                url: '/users/' + $scope.username + '/sessions/' + $scope.sessionId + '/store',
                data: {Name: $scope.sessionId, Content: $scope.sessionContent}
            }).then(function(response){
                $scope.showAlert("success","you have successfully store the session" +
                    "and you can delete it at your accoun center")
            },function(response){
                if (response.status == 409){
                    $scope.showAlert("Max Session reached","Max session num has been reached," +
                        "if you really want to store , please remove previous session  at your account center first")
                }
                if (response.status == 406){
                    $scope.showAlert("same session","The session has been stored already , " +
                        "if you really want to store , please remove previous session  at your account center first")
                }
            }).finally(function(response){
                updateSessionBtnState(false);
            });
        };

        $scope.storeSession = function(){
            if ($scope.instances.length == 0){
                $scope.showAlert("session","no instances have been created")
                return
            }
            $scope.showPrompt(storeSession);
        }

        $scope.showConfirm = function(cb) {
            // Appending dialog to document.body to cover sidenav in docs app
            var confirm = $mdDialog.confirm()
                .title('Would you like to resume your session?')
                .textContent('Current session will be destroyed first and all instances will be deleted' +
                    'if you want to keep it , please store it first ')
                .ok('Please do it!')
                .cancel('Cancel');

            $mdDialog.show(confirm).then(function() {
                if(cb){
                    cb()
                }
            }, function() {
            });
        };

        function resumeSession() {
            updateResumeSessionBtnState(true);
            var session = localStorage.getItem("sessionToResume");
            if (session == null){
                $scope.showAlert("session resume","No session been selected to resume");
                return
            }
            if (session == $scope.sessionId){
                $scope.showAlert("session resume error","cannot resume current session");
                return
            }
            console.log("session to store: "+session);
            window.onbeforeunload = null;
            $scope.socket.emit('session close',$scope.username);
            $http({
                method: 'GET',
                url: '/users/' + $scope.username + '/sessions/' + session + '/resume'
            }).then(function(response){
                $scope.showAlert("success","session resume has been successfully,please wait for a while ...");
                var a = response.data.split(",");
                console.log(a)
                document.write('<a href="/p/'+a[1]+'"'+'>click here jump</a>')
            },function (response) {
                $scope.showAlert("session resume failed","session resume failed");
            }).finally(function () {
                updateResumeSessionBtnState(false);
            })
        }

        $scope.resumeSession = function () {
            var session = localStorage.getItem("sessionToResume");
            if (session == null){
                $scope.showAlert("No session","No such session cound be resumed");
                return
            }
            $scope.showConfirm(resumeSession)
        }

        $scope.newInstance = function() {
            updateNewInstanceBtnState(true);
            var ImageName = InstanceService.getDesiredImage();
            if (ImageName == null){
                alert("please choose the image first")
                updateNewInstanceBtnState(false);
                return
            }
            $http({
                method: 'POST',
                url: '/sessions/' + $scope.sessionId + '/instances/create',
                //TODO:发送mount的请求将目录进行挂载
                data : { ImageName : ImageName }
            }).then(function(response) {
                console.log("new instance info:"+response.data);
                var i = $scope.upsertInstance(response.data);
                $scope.showInstance(i);
            }, function(response) {
                if (response.status == 409) {
                    $scope.showAlert('Max instances reached', 'Maximum number of instances reached')
                }
            }).finally(function() {
                updateNewInstanceBtnState(false);
            });
        }
        $scope.getSession = function(sessionId) {
            $http({
                method: 'GET',
                url: '/sessions/' + $scope.sessionId,
            }).then(function(response) {
                var socket = io({ path: '/sessions/' + sessionId + '/ws' });

                socket.on('terminal out', function(name, data) {
                    var instance = $scope.idx[name];

                    if (!instance) {
                        // instance is new and was created from another client, we should add it
                        $scope.upsertInstance({ name: name ,hostname:hostname,ip:ip});
                        instance = $scope.idx[name];
                        $scope.showInstance(instance);
                    }
                    if (!instance.term) {
                        instance.buffer += data;
                    } else {
                        instance.term.write(data);
                    }
                });

                socket.on('session end', function() {
                    $scope.showAlert('Session end!', 'Your session has ended and all of your instances have been deleted.', '#sessionEnd')
                    $scope.isAlive = false;
                });

                socket.on('new instance', function(name, ip, hostname) {
                    $scope.upsertInstance({ name: name, ip: ip, hostname: hostname });
                    $scope.$apply(function() {
                        if ($scope.instances.length == 1) {
                            $scope.showInstance($scope.instances[0]);
                        }
                    });
                });

                socket.on('delete instance', function(name) {
                    $scope.removeInstance(name);
                    $scope.$apply();
                });
                socket.on('session stored', function(session) {
                    $scope.sessions.push(session);
                });

                socket.on('viewport resize', function(cols, rows) {
                    // viewport has changed, we need to resize all terminals
                    $scope.instances.forEach(function(instance) {
                        instance.term.resize(cols, rows);
                    });
                });

                socket.on('connect_error', function() {
                    $scope.connected = false;
                });
                socket.on('connect', function() {
                    $scope.connected = true;
                });

                $scope.socket = socket;

                var i = response.data;
                for (var k in i.instances) {
                    var instance = i.instances[k];
                    $scope.instances.push(instance);
                    $scope.idx[instance.name] = instance;
                }
                $scope.username = i.user.name;
                $scope.IsTeacher = i.user.is_teacher;
                //TODO:对于以后的experiment content 考虑发起请求获取
                $scope.experiment=i.user.experiment;
                console.log("info from session: "+$scope.username+" "+$scope.IsTeacher+" "+$scope.experiment)
                if(i.user.image_name != ""){
                    console.log("info from session image: "+i.user.image_name)
                    InstanceService.setDesiredImage(i.user.image_name)
                    $scope.ImageName = i.user.image_name;
                }
                for (var i in i.user.sessions){
                    $scope.sessions.push(i);
                }
                // If instance is passed in URL, select it
                let inst = $scope.idx[$location.hash()];
                if (inst) $scope.showInstance(inst);
            }, function(response) {
                if (response.status == 404) {
                    document.write('session not found');
                    return
                }
            });
        }

        $scope.showInstance = function(instance) {
            $scope.selectedInstance = instance;
            $location.hash(instance.name);
            if (!instance.creatingTerminal) {
                if (!instance.term) {
                    $timeout(function () {
                        createTerminal(instance);
                        TerminalService.setFontSize(TerminalService.getFontSize());
                        instance.term.focus();
                    }, 0, false);
                    return
                }
            }
            $timeout(function() {
                instance.term.focus();
            }, 0, false);
        }

        $scope.removeInstance = function(name) {
            if ($scope.idx[name]) {
                delete $scope.idx[name];
                //TODO:2:增加stop 和 start instance功能，整体显示更加友好,webscoket连接建立和回复
                $scope.instances = $scope.instances.filter(function(i) {
                    return i.name != name;
                });
                if ($scope.instances.length) {
                    $scope.showInstance($scope.instances[0]);
                }
            }
        }

        $scope.deleteInstance = function(instance) {
            updateDeleteInstanceBtnState(true);
            $http({
                method: 'DELETE',
                url: '/sessions/' + $scope.sessionId + '/instances/' + instance.name+'/delete',
            }).then(function(response) {
                $scope.removeInstance(instance.name);
            }, function(response) {
                console.log('error', response);
            }).finally(function() {
                updateDeleteInstanceBtnState(false);
            });
        }

        $scope.getSession($scope.sessionId);

        function createTerminal(instance, cb) {
            if (instance.term) {
                return instance.term;
            }

            var terminalContainer = document.getElementById('terminal-' + instance.name);

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

            term.attachCustomKeydownHandler(function(e) {
                if (selectedKeyboardShortcuts == null)
                    return;
                var presets = selectedKeyboardShortcuts.presets
                    .filter(function(preset) { return preset.keyCode == e.keyCode })
                    .filter(function(preset) { return (preset.metaKey == undefined && !e.metaKey) || preset.metaKey == e.metaKey })
                    .filter(function(preset) { return (preset.ctrlKey == undefined && !e.ctrlKey) || preset.ctrlKey == e.ctrlKey })
                    .filter(function(preset) { return (preset.altKey == undefined && !e.altKey) || preset.altKey == e.altKey })
                    .forEach(function(preset) { preset.action({ terminal : term })});
            });

            term.open(terminalContainer);

            // Set geometry during the next tick, to avoid race conditions.
            setTimeout(function() {
                $scope.resize(term.proposeGeometry());
            }, 4);

            term.on('data', function(d) {
                $scope.socket.emit('terminal in', instance.name, d);
            });

            instance.term = term;

            if (instance.buffer) {
                term.write(instance.buffer);
                instance.buffer = '';
            }

            if (cb) {
                cb();
            }
        }

        function updateNewInstanceBtnState(isInstanceBeingCreated) {
            if (isInstanceBeingCreated === true) {
                $scope.newInstanceBtnText = '+ Creating...';
                $scope.isInstanceBeingCreated = true;
            } else {
                $scope.newInstanceBtnText = '+ Add new instance';
                $scope.isInstanceBeingCreated = false;
            }
        }
        function updateSessionBtnState(isSessionBeingStoring) {
            if (isSessionBeingStoring === true) {
                $scope.storeSessionBtnText = 'Storing...';
                $scope.isSessionBeingStoring = true;
            } else {
                $scope.storeSessionBtnText = 'Store';
                $scope.isSessionBeingStoring = false;
            }
        }

        function updateResumeSessionBtnState(isSessionBeingResume) {
            if (isSessionBeingResume === true) {
                $scope.resumeSessionBtnText = 'Resuming...';
                $scope.isSessionBeingResume = true;
            } else {
                $scope.resumeSessionBtnText = 'Resume';
                $scope.isSessionBeingResume = false;
            }
        }

        function updateDeleteInstanceBtnState(isInstanceBeingDeleted) {
            if (isInstanceBeingDeleted === true) {
                $scope.deleteInstanceBtnText = 'Deleting...';
                $scope.isInstanceBeingDeleted = true;
            } else {
                $scope.deleteInstanceBtnText = 'Delete';
                $scope.isInstanceBeingDeleted = false;
            }
        }
    }])
        .config(['$mdIconProvider', '$locationProvider', function($mdIconProvider, $locationProvider) {
            $locationProvider.html5Mode({enabled: true, requireBase: false});
            $mdIconProvider.defaultIconSet('../assets/social-icons.svg', 24);
        }])
        .component('settingsIcon', {
            template : "<md-button class='md-mini' ng-click='$ctrl.onClick()'><md-icon class='material-icons'>settings</md-icon></md-button>",
            controller : function($mdDialog) {
                var $ctrl = this;
                $ctrl.onClick = function() {
                    $mdDialog.show({
                        controller : function() {},
                        template : "<settings-dialog></settings-dialog>",
                        parent: angular.element(document.body),
                        clickOutsideToClose : true
                    })
                }
            }
        })
        .component("settingsDialog", {
            templateUrl : "settings-modal.html",
            controller : function($mdDialog,KeyboardShortcutService, $rootScope,$scope, InstanceService, TerminalService) {
                var $ctrl = this;
                $ctrl.$onInit = function() {
                    $ctrl.keyboardShortcutPresets = KeyboardShortcutService.getAvailablePresets();
                    $ctrl.selectedShortcutPreset = KeyboardShortcutService.getCurrentShortcuts();
                    $ctrl.terminalFontSizes = TerminalService.getFontSizes();
                    $scope.isImageSearched = false ;
                    $ctrl.imageLimits = $ctrl.getImageLimits();
                };

                $ctrl.currentShortcutConfig = function(value) {
                    if (value !== undefined) {
                        value = JSON.parse(value);
                        KeyboardShortcutService.setCurrentShortcuts(value);
                        $ctrl.selectedShortcutPreset = angular.copy(KeyboardShortcutService.getCurrentShortcuts());
                        $rootScope.$broadcast('settings:shortcutsSelected', $ctrl.selectedShortcutPreset);
                    }
                    return JSON.stringify(KeyboardShortcutService.getCurrentShortcuts());
                };

                $ctrl.currentDesiredInstanceImage = function(value) {
                    if (value !== undefined) {
                        InstanceService.setDesiredImage(value);
                    }
                    return InstanceService.getDesiredImage(value);
                };
                $ctrl.currentTerminalFontSize = function(value) {
                    if (value !== undefined) {
                        // set font size
                        TerminalService.setFontSize(value);
                        return;
                    }

                    return TerminalService.getFontSize();
                };

                $ctrl.imageLimit = function (value){
                    if (value !== undefined){
                        localStorage.setItem("settings.imageLimit",value) ;
                        return;
                    }
                    var imageLimit = localStorage.getItem("settings.imageLimit") ;
                    if (imageLimit == null){
                        return 5 ;
                    }
                    return imageLimit;
                };
                $ctrl.getImageLimits = function (){
                    var imageLimits = []
                    for (var i = 25; i >= 0; i=i-5) {
                        imageLimits.push(i);
                    }
                    return imageLimits;
                };
                $ctrl.ImageSearch = function(term,limitNum){
                    if (term !== undefined || limitNum !== undefined){
                        var value = {
                            term:term,
                            limitNum:limitNum,
                        }
                        InstanceService.prepopulateAvailableImages(value,function(){
                            $ctrl.instanceImages = InstanceService.getAvailableImages();
                            $scope.isImageSearched = true ;
                        })
                    }else{
                        alert("please set the searching property");
                        return ;
                    }
                }

                $ctrl.close = function() {
                    $mdDialog.cancel();
                }
            }
        })
        .service("InstanceService", function($http) {
            var instanceImages = [];
            //prepopulateAvailableImages();

            return {
                getAvailableImages : getAvailableImages,
                setDesiredImage : setDesiredImage,
                getDesiredImage : getDesiredImage,
                prepopulateAvailableImages : prepopulateAvailableImages,
            };

            function getAvailableImages() {
                return instanceImages;
            }

            function getDesiredImage() {
                var image = localStorage.getItem("settings.desiredImage");
                if (image == null)
                    return ;
                return image;
            }

            function setDesiredImage(image) {
                if (image == null)
                    localStorage.removeItem("settings.desiredImage");
                else
                    localStorage.setItem("settings.desiredImage", image);
            }

            function prepopulateAvailableImages(value,cb) {
                $http({
                    method: 'POST',
                    url: '/images/search',
                    data : { Term : value.term, LimitNum :parseInt(value.limitNum)  }
                }).then(function(response) {
                    instanceImages = response.data;
                    console.log(instanceImages);
                    if(cb) cb();
                    setDesiredImage(instanceImages[0]);
                }, function(response) {
                    alert("no such image could found")
                })
            }
        })
        .service("KeyboardShortcutService", ['TerminalService', function(TerminalService) {
            var resizeFunc;

            return {
                getAvailablePresets : getAvailablePresets,
                getCurrentShortcuts : getCurrentShortcuts,
                setCurrentShortcuts : setCurrentShortcuts,
                setResizeFunc : setResizeFunc
            };

            function setResizeFunc(f) {
                resizeFunc = f;
            }

            function getAvailablePresets() {
                return [
                    { name : "None", presets : [
                        { description : "Toggle terminal fullscreen", command : "Alt+enter", altKey : true, keyCode : 13, action : function(context) { TerminalService.toggleFullscreen(context.terminal, resizeFunc); }}
                    ] },
                    {
                        name : "Mac OSX",
                        presets : [
                            { description : "Clear terminal", command : "Cmd+K", metaKey : true, keyCode : 75, action : function(context) { context.terminal.clear(); }},
                            { description : "Toggle terminal fullscreen", command : "Alt+enter", altKey : true, keyCode : 13, action : function(context) { TerminalService.toggleFullscreen(context.terminal, resizeFunc); }}
                        ]
                    }
                ]
            }

            function getCurrentShortcuts() {
                var shortcuts = localStorage.getItem("shortcut-preset-name");
                if (shortcuts == null) {
                    shortcuts = getDefaultShortcutPrefixName();
                    if (shortcuts == null)
                        return null;
                }

                var preset = getAvailablePresets()
                    .filter(function(preset) { return preset.name == shortcuts; });
                if (preset.length == 0)
                    console.error("Unable to find preset with name '" + shortcuts + "'");
                return preset[0];
                return (shortcuts == null) ? null : JSON.parse(shortcuts);
            }

            function setCurrentShortcuts(config) {
                localStorage.setItem("shortcut-preset-name", config.name);
            }

            function getDefaultShortcutPrefixName() {
                if (window.navigator.platform.toUpperCase().indexOf('MAC') >= 0)
                    return "Mac OSX";
                return "None";
            }
        }])
        .service('TerminalService', ['$window', function($window) {
            var fullscreen;
            var fontSize = getFontSize();
            return {
                getFontSizes : getFontSizes,
                setFontSize : setFontSize,
                getFontSize : getFontSize,
                increaseFontSize : increaseFontSize,
                decreaseFontSize : decreaseFontSize,
                toggleFullscreen : toggleFullscreen
            };
            function getFontSizes() {
                var terminalFontSizes = [];
                for (var i=3; i<40; i++) {
                    terminalFontSizes.push(i+'px');
                }
                return terminalFontSizes;
            };
            function getFontSize() {
                if (!fontSize) {
                    return $('.terminal').css('font-size');
                }
                return fontSize;
            }
            function setFontSize(value) {
                fontSize = value;
                var size = parseInt(value);
                $('.terminal').css('font-size', value).css('line-height', (size + 2)+'px');
                //.css('line-height', value).css('height', value);
                angular.element($window).trigger('resize');
            }
            function increaseFontSize() {
                var sizes = getFontSizes();
                var size = getFontSize();
                var i = sizes.indexOf(size);
                if (i == -1) {
                    return;
                }
                if (i+1 > sizes.length) {
                    return;
                }
                setFontSize(sizes[i+1]);
            }
            function decreaseFontSize() {
                var sizes = getFontSizes();
                var size = getFontSize();
                var i = sizes.indexOf(size);
                if (i == -1) {
                    return;
                }
                if (i-1 < 0) {
                    return;
                }
                setFontSize(sizes[i-1]);
            }
            function toggleFullscreen(terminal, resize) {
                if(fullscreen) {
                    terminal.toggleFullscreen();
                    resize(fullscreen);
                    fullscreen = null;
                } else {
                    fullscreen = terminal.proposeGeometry();
                    terminal.toggleFullscreen();
                    angular.element($window).trigger('resize');
                }
            }
        }]);
})();
