var baseURL = "http://localhost:1337/localhost:8080/apis";
var APIGroup = "nerdalize.com";
var APIVersion = "v1alpha1";

$(function(){ // on dom ready
  if(window.location.hash == "") {
    alert("Please select workflow in URL hash [namespace/wf-name]");
    return
  }

  var parts = window.location.hash.replace("#", '').split("/");
  var url = baseURL + "/" + APIGroup + "/" + APIVersion + "/namespaces/" + parts[0] + "/workflows/" + parts[1];
  console.log(url);
  $.getJSON(url, function (data) {
    if(data.kind == "Status") {
      alert("Received a status message: " + data)
    } else if (data.kind == "Workflow") {
      console.log(data);
      var nodes = [];
      var edges = [];
      $.each(data.spec.steps, function(name, step) {
        nodes.push({data: {id: name}})
        if(step.dependencies !== undefined) {
          $.each(step.dependencies, function(idx, dep) {
            edges.push({data: {
              source: dep,
              target: name
            }});
          });
        }
      });
      var cy = cytoscape({
        container: document.getElementById('cy'),

        boxSelectionEnabled: false,
        autounselectify: true,

        style: cytoscape.stylesheet()
          .selector('node')
            .css({
              'content': 'data(id)'
            })
          .selector('edge')
            .css({
              'target-arrow-shape': 'triangle',
              'width': 4,
              'line-color': '#ddd',
              'target-arrow-color': '#ddd'
            })
          .selector('.running')
            .css({
              'background-color': '#61bffc',
              'line-color': '#61bffc',
              'target-arrow-color': '#61bffc',
              'transition-property': 'background-color, line-color, target-arrow-color',
              'transition-duration': '0.5s'
            })
          .selector('.completed')
            .css({
              'background-color': 'green',
              'line-color': 'green',
              'target-arrow-color': 'green',
              'transition-property': 'background-color, line-color, target-arrow-color',
              'transition-duration': '0.5s'
            }),

        elements: {
            nodes: nodes,
            edges: edges
          },

        layout: {
          name: 'dagre',
          fit: true,
          rankSep: 1250,
          edgeSep: 25
        }
      });
      cy.center();
      cy.fit();
      var highlightNextEle = function(){
        console.log("P");
        $.getJSON(url, function (data) {
          if(data.kind == "Workflow") {
            if(data.status !== undefined && data.status.statuses !== undefined) {
              $.each(data.status.statuses, function(step, status) {
                if(status.complete === true) {
                    cy.$('#' + step).addClass('completed')
                } else {
                  cy.$('#' + step).addClass('running')
                }
              });
            }
          }
          setTimeout(highlightNextEle, 1000);
        });
      };
      highlightNextEle();
    }
  });



}); // on dom ready
