function q(n,g,y,r){let h=n[g],x=n[g+1],t=0;for(let c=g+r;c<y;c+=r){let u=n[c],p=n[c+1];t+=Math.sqrt((u-h)*(u-h)+(p-x)*(p-x)),h=u,x=p}return t}export{q as a};
