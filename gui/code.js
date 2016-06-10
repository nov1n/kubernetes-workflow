// We use a proxy on 1337 to deal with CORS. Localhost:8080 is (a proxy to)
// the actual API server.
var baseURL = "http://localhost:1337/localhost:8080/apis";
// APIGroup and APIVersion are used to query the correct API endpoint.
var APIGroup = "nerdalize.com";
var APIVersion = "v1alpha1";

// graphStylesheet is the stylesheet to style graphs in cytoscape.
var graphStylesheet = cytoscape.stylesheet()
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
    });


// nodesAndEdgesFromData extracts a list of nodes and a list of edges from the
// data received from the API server. In the API data edges are represented as
// dependencies on a node.
function nodesAndEdgesFromData(data) {
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
  return {
    nodes: nodes,
    edges: edges
  };
}

// updateGraph highlights the running and completed steps in the graph.
// updateGraph runs every second.
function updateGraph(url){
  console.log("P");
  $.getJSON(url, function (data) {
    if(data.kind == "Workflow") {
      if(data.status !== undefined && data.status.statuses !== undefined) {
        $.each(data.status.statuses, function(step, status) {
          if(status.complete === true) {
            cy.$('#' + step).addClass('completed')
            cy.$('edge[source="' + step + '"]').addClass('completed')
          } else {
            cy.$('#' + step).addClass('running')
            cy.$('edge[source="' + step + '"]').addClass('running')
          }
        });
      }
    }
    setTimeout(updateGraph(url), 1000);
  });
};

// getUrl returns the URL for the workflow endpoint.
function getUrl() {
  var parts = window.location.hash.replace("#", '').split("/");
  return baseURL + "/" + APIGroup + "/" + APIVersion + "/namespaces/" + parts[0] + "/workflows/" + parts[1];
}

// createGraph initializes the graph and calls the update function.
function createGraph(url) {
  $.getJSON(url, function (data) {
    if(data.kind == "Status") {
      alert("Received a status message: " + data)
    } else if (data.kind == "Workflow") {
      console.log(data);
      var cy = cytoscape({
        container: document.getElementById('cy'),
        boxSelectionEnabled: false,
        autounselectify: true,
        style: graphStylesheet,
        elements: nodesAndEdgesFromData(data),
        layout: {
          name: 'dagre',
          fit: true,
          rankSep: 1250,
          edgeSep: 25
        }
      });
      updateGraph(url);
    }
  });
}

// on dom ready
$(function(){
  // The workflow should be specified using a hash.
  if(window.location.hash == "") {
    alert("Please select workflow in URL hash [namespace/wf-name]");
    return
  }
  // Create the graph.
  var url = getUrl();
  console.log(url);
  createGraph(url);
});
