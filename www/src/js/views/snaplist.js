// snaplist view
var _ = require('lodash');
var $ = require('jquery');
var Backbone = require('backbone');
Backbone.$ = $;
var Marionette = require('backbone.marionette');
var SnaplistItemView = require('../views/snaplist-item.js');
var EmptySnaplistView = require('./empty-snaplist.js');
var template = require('../templates/snaplist.hbs');
var CONF = require('../config.js');
var SnapTools = require('../common/snaps.js')
var Radio = require('backbone.radio');
var chan = Radio.channel('root');

module.exports = Marionette.CompositeView.extend({

  childViewContainer: '#js-snaplist',

  initialize : function(options) {
    if (options.doNotDisplayEmptyList) {
      this.emptyView = null;
    }
    // "patch" the model with the snap details browse URI
    this.collection.each(function(snap) {
      snap.set('targetSnapUri', SnapTools.getShowSnapUrlFor(snap))
    });
    chan.comply('snap:removed', this.snapRemoved, this);
  },

  snapRemoved: function(model) {
    this.collection.remove(this.collection.get(model.get('id')));
    this.render()
  },

  template : function(model) {
    return template(model);
  },

  childViewOptions: function(model, index) {
    var lastCol = (index != 0 && ((index + 1) % 4) == 0);

    return {
      lastCol: lastCol
    };
  },

  childView: SnaplistItemView,

  emptyView: EmptySnaplistView
});
