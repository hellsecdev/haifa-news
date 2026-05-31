<?php
require_once '/var/www/html/wp-load.php';
function clean_html($s){ return apply_filters('the_content', $s); }
function local_media_url($url){
  if (!$url) return '';
  $p = parse_url($url, PHP_URL_PATH);
  if ($p && strpos($p, '/wp-content/uploads/') === 0) return str_replace('/wp-content/uploads/', '/uploads/', $p);
  return $url;
}
$cats=[];
foreach (get_categories(['hide_empty'=>false]) as $c) {
  if ($c->slug === 'uncategorized' && intval($c->count)===0) continue;
  $cats[]=['wp_id'=>$c->term_id,'name'=>$c->name,'slug'=>rawurldecode($c->slug),'count'=>intval($c->count)];
}
$posts=[];
$q=new WP_Query(['post_type'=>'post','post_status'=>'publish','posts_per_page'=>-1,'orderby'=>'date','order'=>'DESC','no_found_rows'=>true]);
foreach ($q->posts as $p) {
  $pc=get_the_category($p->ID); $cat_id=$pc ? $pc[0]->term_id : null;
  $thumb=get_the_post_thumbnail_url($p->ID,'full');
  $excerpt=trim(get_the_excerpt($p));
  if (!$excerpt) $excerpt=wp_trim_words(wp_strip_all_tags($p->post_content),32,'…');
  $slug=rawurldecode($p->post_name ?: sanitize_title($p->post_title));
  $posts[]=[
    'wp_id'=>$p->ID,
    'title'=>get_the_title($p),
    'slug'=>$slug,
    'excerpt'=>$excerpt,
    'content'=>clean_html($p->post_content),
    'featured_image'=>local_media_url($thumb),
    'category_wp_id'=>$cat_id,
    'published_at'=>get_post_time('c', true, $p),
    'updated_at'=>get_post_modified_time('c', true, $p),
    'status'=>'publish'
  ];
}
echo json_encode(['exported_at'=>gmdate('c'),'categories'=>$cats,'posts'=>$posts], JSON_UNESCAPED_UNICODE|JSON_UNESCAPED_SLASHES|JSON_PRETTY_PRINT);
